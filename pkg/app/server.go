package app

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
	"go.etcd.io/etcd/pkg/v3/idutil"
	"go.etcd.io/etcd/pkg/v3/wait"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.uber.org/atomic"
	"go.uber.org/zap"

	"github.com/gozssky/groupchat/pkg/future"
	"github.com/gozssky/groupchat/pkg/raftnode"
	"github.com/gozssky/groupchat/pkg/storage"
)

type Server struct {
	lg       *zap.Logger
	port     int
	raftPort int
	dataDir  string
	aead     cipher.AEAD

	once           sync.Once
	node           *raftnode.Node
	clusterStarted atomic.Bool

	rwm     sync.RWMutex
	storage *storage.Storage

	reqIDGen     *idutil.Generator
	readWaitC    chan struct{}
	readyReadMu  sync.RWMutex
	readyRead    *future.Result
	applyWait    wait.WaitTime
	applyNotify  wait.Wait
	appliedIndex atomic.Uint64
}

func NewServer(lg *zap.Logger, port, raftPort int, dataDir string) *Server {
	return &Server{
		lg:       lg,
		port:     port,
		raftPort: raftPort,
		dataDir:  dataDir,
		storage:  storage.NewStorage(),
	}
}

func (s *Server) Run() error {
	chatAPI := s.newChatHandler()
	node, ok := raftnode.RestartRaftNode(s.lg, s.dataDir)
	if ok {
		s.lg.Info("restart the existing raft cluster")
		go s.bootstrap(func() *raftnode.Node { return node })
	}
	return fasthttp.ListenAndServe(fmt.Sprintf(":%d", s.port), chatAPI)
}

func (s *Server) initAEAD() {
	secretKey := s.getOrInitSecretKey()
	block, err := aes.NewCipher(secretKey)
	if err != nil {
		s.lg.Panic("failed to create aes cipher block", zap.Error(err))
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		s.lg.Panic("failed to create gcm aead", zap.Error(err))
	}
	s.aead = aead
	s.lg.Info("AEAD cipher mode is initialized")
}

func (s *Server) getSecretKey(ctx context.Context) ([]byte, error) {
	if err := s.linearizableReadNotify(ctx); err != nil {
		return nil, err
	}
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	return append([]byte(nil), s.storage.SecretKey...), nil
}

func (s *Server) getOrInitSecretKey() []byte {
	for ; ; time.Sleep(time.Second) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		secretKey, err := s.getSecretKey(ctx)
		cancel()
		if err != nil {
			s.lg.Warn("failed to get secret key", zap.Error(err))
			continue
		}
		if len(secretKey) > 0 {
			return secretKey
		}
		if !s.node.IsLead() {
			s.lg.Info("secret key is empty, wait leader to initialize")
			continue
		}
		s.lg.Info("secret key is empty, start to initialize new secret key")
		secretKey = make([]byte, aes.BlockSize)
		rand.Read(secretKey)
		ctx, cancel = context.WithTimeout(context.Background(), time.Second*5)
		result, err := s.proposeRaftCommand(ctx, storage.InternalRaftCommand{
			InitSecretKey: &storage.InitSecretKeyCommand{
				SecretKey: secretKey,
			},
		})
		cancel()
		if err != nil {
			s.lg.Warn("failed to initialize secret key", zap.Error(err))
			continue
		}
		return result.([]byte)
	}
}

func (s *Server) bootstrap(newRaftNode func() *raftnode.Node) {
	s.once.Do(func() {
		s.node = newRaftNode()
		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", s.raftPort), s.node.Handler()); err != nil {
				s.lg.Fatal("failed to run raft server")
			}
		}()
		s.reqIDGen = idutil.NewGenerator(uint16(s.node.ID()), time.Now())
		s.readWaitC = make(chan struct{}, 1)
		s.readyRead = future.NewResult()
		s.applyWait = wait.NewTimeList()
		s.applyNotify = wait.New()
		go s.handleApplyTasks()
		go s.linearizableReadLoop()
		s.initAEAD()
		s.clusterStarted.Store(true)
	})
}

func (s *Server) applyAll(entries []raftpb.Entry) {
	if len(entries) == 0 {
		return
	}
	s.rwm.RLock()
	lastIndex := s.storage.Index
	s.rwm.RUnlock()

	newIndex := lastIndex
	var commands []storage.InternalRaftCommand
	for _, entry := range entries {
		if entry.Index <= lastIndex {
			continue
		}
		newIndex = entry.Index
		if len(entry.Data) == 0 {
			continue
		}
		var cmd storage.InternalRaftCommand
		cmd.MustUnmarshalGOB(entry.Data)
		commands = append(commands, cmd)
	}

	s.rwm.Lock()
	for _, cmd := range commands {
		result := cmd.Execute(s.storage)
		s.applyNotify.Trigger(cmd.ID, result)
	}
	s.storage.Index = newIndex
	s.appliedIndex.Store(newIndex)
	s.rwm.Unlock()
	s.applyWait.Trigger(newIndex)
}

func (s *Server) applySnapshot(snap raftpb.Snapshot) {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	s.storage.RecoverFromSnapshot(snap.Data)
	s.appliedIndex.Store(snap.Metadata.Index)
}

func (s *Server) handleApplyTasks() {
	for task := range s.node.ApplyTasks() {
		if !raft.IsEmptySnap(task.Snapshot) {
			s.applySnapshot(task.Snapshot)
		}
		s.applyAll(task.Entries)
	}
}

func (s *Server) proposeRaftCommand(
	ctx context.Context,
	cmd storage.InternalRaftCommand,
) (result interface{}, err error) {
	cmd.ID = s.reqIDGen.Next()
	notify := s.applyNotify.Register(cmd.ID)
	if err := s.node.Propose(ctx, cmd.MustMarshalGOB()); err != nil {
		s.applyNotify.Trigger(cmd.ID, nil)
		return nil, err
	}
	select {
	case v := <-notify:
		execRes := v.(*storage.ExecuteResult)
		return execRes.Result, execRes.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Server) applyToLatest(ctx context.Context) error {
	id := s.reqIDGen.Next()
	ctxToSend := make([]byte, 8)
	binary.BigEndian.PutUint64(ctxToSend, id)

	if err := s.node.ReadIndex(ctx, ctxToSend); err != nil {
		return err
	}
	var rs raft.ReadState
	for done := false; !done; {
		select {
		case rs = <-s.node.ReadStates():
			done = bytes.Equal(rs.RequestCtx, ctxToSend)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if s.appliedIndex.Load() < rs.Index {
		select {
		case <-s.applyWait.Wait(rs.Index):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *Server) linearizableReadLoop() {
	for {
		<-s.readWaitC
		s.readyReadMu.Lock()
		readyRead := s.readyRead
		s.readyRead = future.NewResult()
		s.readyReadMu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		err := s.applyToLatest(ctx)
		cancel()
		readyRead.Notify(err)
	}
}

func (s *Server) linearizableReadNotify(ctx context.Context) error {
	s.readyReadMu.RLock()
	readyRead := s.readyRead
	s.readyReadMu.RUnlock()

	select {
	case s.readWaitC <- struct{}{}:
	default:
	}
	select {
	case <-readyRead.Done():
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}
