package raftnode

import (
	"context"
	"go.uber.org/atomic"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	stats "go.etcd.io/etcd/server/v3/etcdserver/api/v2stats"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
	"go.uber.org/zap"

	"github.com/gozssky/groupchat/pkg/metadata"
)

type ApplyTask struct {
	Snapshot raftpb.Snapshot
	Entries  []raftpb.Entry
}

type Node struct {
	lg          *zap.Logger
	id          types.ID
	lead        atomic.Uint64
	node        raft.Node
	storage     *raft.MemoryStorage
	wal         *wal.WAL
	snapshotter *snap.Snapshotter
	transport   rafthttp.Transporter

	applyTaskC chan ApplyTask
	readStateC chan raft.ReadState
}

func NewRaftNode(lg *zap.Logger, localURL string, remoteURLs []string, dataDir string) *Node {
	snapDir := filepath.Join(dataDir, "snap")
	walDir := filepath.Join(dataDir, "wal")
	ensureEmptyDir(lg, snapDir)
	ensureEmptyDir(lg, walDir)

	storage := raft.NewMemoryStorage()
	peerURLs := []string{localURL}
	peerURLs = append(peerURLs, remoteURLs...)
	sort.Strings(peerURLs)
	id := types.ID(sort.Search(len(peerURLs), func(i int) bool { return peerURLs[i] >= localURL }) + 1)
	raftCfg := &raft.Config{
		ID:                        uint64(id),
		ElectionTick:              10,
		HeartbeatTick:             1,
		Storage:                   storage,
		MaxSizePerMsg:             1024 * 1024,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
		PreVote:                   true,
		Logger:                    newRaftLogger(lg),
	}

	var raftPeers []raft.Peer
	for i := 1; i <= len(peerURLs); i++ {
		raftPeers = append(raftPeers, raft.Peer{ID: uint64(i)})
	}
	node := raft.StartNode(raftCfg, raftPeers)

	md := &metadata.Metadata{ID: id}
	for i, url := range peerURLs {
		md.Peers = append(md.Peers, metadata.Peer{ID: types.ID(i + 1), URL: url})
	}
	w, err := wal.Create(lg, walDir, md.MustMarshalJSON())
	if err != nil {
		lg.Fatal("failed to create wal", zap.Error(err))
	}
	snapshotter := snap.New(lg, snapDir)

	rc := &Node{
		lg:          lg,
		id:          id,
		node:        node,
		storage:     storage,
		wal:         w,
		snapshotter: snapshotter,
		applyTaskC:  make(chan ApplyTask),
		readStateC:  make(chan raft.ReadState, 1),
	}
	transport := &rafthttp.Transport{
		Logger:      lg,
		ID:          id,
		ClusterID:   0x1000,
		Raft:        httpRaft{Node: node},
		ServerStats: stats.NewServerStats("", ""),
		LeaderStats: stats.NewLeaderStats(zap.NewExample(), strconv.Itoa(int(id))),
		ErrorC:      make(chan error),
	}
	if err := transport.Start(); err != nil {
		lg.Fatal("failed to start transport", zap.Error(err))
	}
	for i, url := range peerURLs {
		if i+1 != int(id) {
			transport.AddPeer(types.ID(i+1), []string{url})
		}
	}
	rc.transport = transport
	go rc.serveRaft()
	return rc
}

func RestartRaftNode(lg *zap.Logger, dataDir string) (*Node, bool) {
	snapDir := filepath.Join(dataDir, "snap")
	walDir := filepath.Join(dataDir, "wal")
	ensureDir(lg, snapDir)
	ensureDir(lg, walDir)

	if !wal.Exist(walDir) {
		return nil, false
	}

	snapshotter := snap.New(lg, snapDir)
	walSnaps, err := wal.ValidSnapshotEntries(lg, walDir)
	if err != nil {
		lg.Fatal("failed to read valid wal snapshot", zap.Error(err))
	}
	raftSnap, err := snapshotter.LoadNewestAvailable(walSnaps)
	if err != nil && err != snap.ErrNoSnapshot {
		lg.Fatal("failed to load newest raft snapshot", zap.Error(err))
	}

	// Open wal at the given snapshot.
	var walSnap walpb.Snapshot
	if raftSnap != nil {
		walSnap = walpb.Snapshot{
			Index: raftSnap.Metadata.Index,
			Term:  raftSnap.Metadata.Term,
		}
	}
	w, err := wal.Open(lg, walDir, walSnap)
	if err != nil {
		lg.Fatal("failed to load wal", zap.Error(err))
	}

	// Replay wal and append to storage.
	rawMetadata, st, ents, err := w.ReadAll()
	if err != nil {
		lg.Fatal("failed to read all entries from wal", zap.Error(err))
	}
	var md metadata.Metadata
	md.MustUnmarshalJSON(rawMetadata)
	storage := raft.NewMemoryStorage()
	if raftSnap != nil {
		storage.ApplySnapshot(*raftSnap)
	}
	storage.SetHardState(st)
	storage.Append(ents)
	lastIndex, err := storage.LastIndex()
	if err != nil {
		lg.Fatal("failed to get last index from storage", zap.Error(err))
	}
	if int(lastIndex) < len(md.Peers) {
		return nil, false
	}

	raftCfg := &raft.Config{
		ID:                        uint64(md.ID),
		ElectionTick:              10,
		HeartbeatTick:             1,
		Storage:                   storage,
		MaxSizePerMsg:             1024 * 1024,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
		PreVote:                   true,
		Logger:                    newRaftLogger(lg),
	}
	node := raft.RestartNode(raftCfg)

	rc := &Node{
		lg:          lg,
		id:          md.ID,
		node:        node,
		storage:     storage,
		wal:         w,
		snapshotter: snapshotter,
		applyTaskC:  make(chan ApplyTask),
		readStateC:  make(chan raft.ReadState, 1),
	}
	idStr := strconv.Itoa(int(md.ID))
	transport := &rafthttp.Transport{
		Logger:      lg,
		ID:          md.ID,
		ClusterID:   0x1000,
		Raft:        httpRaft{Node: node},
		ServerStats: stats.NewServerStats(idStr, idStr),
		LeaderStats: stats.NewLeaderStats(lg, idStr),
		ErrorC:      make(chan error),
	}
	if err := transport.Start(); err != nil {
		lg.Fatal("failed to start transport", zap.Error(err))
	}
	for _, peer := range md.Peers {
		if peer.ID != md.ID {
			transport.AddPeer(peer.ID, []string{peer.URL})
		}
	}
	rc.transport = transport
	go rc.serveRaft()
	return rc, true
}

func (rc *Node) ID() types.ID {
	return rc.id
}

func (rc *Node) IsLead() bool {
	return rc.lead.Load() == uint64(rc.id)
}

func (rc *Node) Handler() http.Handler {
	return rc.transport.Handler()
}

func (rc *Node) saveSnap(snap raftpb.Snapshot) error {
	walSnap := walpb.Snapshot{
		Index:     snap.Metadata.Index,
		Term:      snap.Metadata.Term,
		ConfState: &snap.Metadata.ConfState,
	}
	if err := rc.snapshotter.SaveSnap(snap); err != nil {
		return err
	}
	if err := rc.wal.SaveSnapshot(walSnap); err != nil {
		return err
	}
	return rc.wal.ReleaseLockTo(snap.Metadata.Index)
}

func (rc *Node) serveRaft() {
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rc.node.Tick()
		case rd := <-rc.node.Ready():
			if rd.SoftState != nil {
				rc.lead.Store(rd.SoftState.Lead)
			}
			if len(rd.ReadStates) != 0 {
				rc.readStateC <- rd.ReadStates[len(rd.ReadStates)-1]
			}
			task := ApplyTask{Snapshot: rd.Snapshot}
			sawNormal := false
			for _, entry := range rd.CommittedEntries {
				switch entry.Type {
				case raftpb.EntryNormal:
					task.Entries = append(task.Entries, entry)
					sawNormal = true
				case raftpb.EntryConfChange:
					if sawNormal {
						rc.lg.Fatal("unexpected ConfChange entry")
					}
					var cc raftpb.ConfChange
					pbutil.MustUnmarshal(&cc, entry.Data)
					rc.node.ApplyConfChange(cc)
				default:
					rc.lg.Fatal("unknown raft entry type", zap.Stringer("type", entry.Type))
				}
			}
			rc.applyTaskC <- task
			if !raft.IsEmptySnap(rd.Snapshot) {
				if err := rc.saveSnap(rd.Snapshot); err != nil {
					rc.lg.Fatal("failed to save snapshot", zap.Error(err))
				}
				rc.storage.ApplySnapshot(rd.Snapshot)
			}
			if err := rc.wal.Save(rd.HardState, rd.Entries); err != nil {
				rc.lg.Fatal("failed to save raft entries", zap.Error(err))
			}
			rc.storage.Append(rd.Entries)
			rc.transport.Send(rd.Messages)
			rc.node.Advance()
		}
	}
}

func (rc *Node) ApplyTasks() <-chan ApplyTask {
	return rc.applyTaskC
}

func (rc *Node) ReadStates() <-chan raft.ReadState {
	return rc.readStateC
}

func (rc *Node) ReadIndex(ctx context.Context, rctx []byte) error {
	return rc.node.ReadIndex(ctx, rctx)
}

func (rc *Node) Propose(ctx context.Context, data []byte) error {
	return rc.node.Propose(ctx, data)
}
