package app

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/gozssky/groupchat/pkg/raftnode"
	"github.com/gozssky/groupchat/pkg/storage"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func writeError(ctx *fasthttp.RequestCtx, err error) {
	ctx.SetStatusCode(fasthttp.StatusBadRequest)
	ctx.SetContentType("text/plain")
	ctx.WriteString(err.Error())
}

func writeJSON(ctx *fasthttp.RequestCtx, obj interface{}) {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	ctx.Write(data)
}

func bindJSON(ctx *fasthttp.RequestCtx, obj interface{}) bool {
	if err := json.Unmarshal(ctx.PostBody(), obj); err != nil {
		writeError(ctx, err)
		return false
	}
	return true
}

func (s *Server) handleClusterUpdate(ctx *fasthttp.RequestCtx) {
	var clusterIPs []string
	if !bindJSON(ctx, &clusterIPs) {
		return
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		writeError(ctx, err)
		return
	}

	var localIP string
outer:
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipv4 := ipNet.IP.To4(); ipv4 != nil {
				for _, ip := range clusterIPs {
					if ipv4.String() == ip {
						localIP = ip
						break outer
					}
				}
			}
		}
	}
	if len(localIP) == 0 {
		writeError(ctx, errors.New("local ip not exists in cluster"))
		return
	}
	//goland:noinspection HttpUrlsUsage
	localURL := fmt.Sprintf("http://%s:%d", localIP, s.raftPort)
	//goland:noinspection HttpUrlsUsage
	localClusterURL := fmt.Sprintf("http://%s:%d", localIP, s.port)
	var remoteURLs []string
	for _, ip := range clusterIPs {
		if ip == localIP {
			continue
		}
		//goland:noinspection HttpUrlsUsage
		remoteURL := fmt.Sprintf("http://%s:%d", ip, s.raftPort)
		remoteURLs = append(remoteURLs, remoteURL)
		//goland:noinspection HttpUrlsUsage
		remoteClusterURL := fmt.Sprintf("http://%s:%d/updateCluster", ip, s.port)
		if len(ctx.Request.Header.Peek("Referer")) == 0 {
			go func() {
				data, err := json.Marshal(clusterIPs)
				if err != nil {
					panic(err)
				}
				req, err := http.NewRequest(http.MethodPost, remoteClusterURL, bytes.NewReader(data))
				if err != nil {
					panic(err)
				}
				req.Header.Add("Referer", localClusterURL)
				logger := s.lg.With(zap.String("remote-cluster-url", remoteClusterURL))
				logger.Info("forward updateCluster request to remote")
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					logger.Warn("failed to forward updateCluster request", zap.Error(err))
					return
				}
				if resp.StatusCode != http.StatusOK {
					logger.Warn(
						"forwarding request received unexpected status code",
						zap.String("status", resp.Status),
						zap.Int("status-code", resp.StatusCode),
					)
				}
			}()
		}
	}
	s.lg.Info(
		"start to bootstrap a new raft cluster",
		zap.String("local-url", localURL),
		zap.Strings("remote-urls", remoteURLs),
	)
	go s.bootstrap(func() *raftnode.Node {
		return raftnode.NewRaftNode(s.lg, localURL, remoteURLs, s.dataDir)
	})
}

func (s *Server) handleClusterCheck(_ *fasthttp.RequestCtx) {}

func (s *Server) handleUserCreate(ctx *fasthttp.RequestCtx) {
	var user struct {
		UserName  string `json:"username"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		Phone     string `json:"phone"`
	}
	if !bindJSON(ctx, &user) {
		return
	}
	if _, err := s.proposeRaftCommand(ctx, storage.InternalRaftCommand{
		CreateUser: &storage.CreateUserCommand{
			UserName:  user.UserName,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Password:  user.Password,
			Phone:     user.Phone,
		},
	}); err != nil {
		writeError(ctx, err)
		return
	}
}

func (s *Server) handleUserQuery(ctx *fasthttp.RequestCtx) {
	name := strings.TrimPrefix(string(ctx.Path()), "/user/")
	s.rwm.RLock()
	defer s.rwm.RUnlock()

	user, ok := s.storage.Users[name]
	if !ok {
		writeError(ctx, errors.New("user not exists"))
		return
	}
	writeJSON(ctx, map[string]string{
		"firstName": user.FirstName,
		"lastName":  user.LastName,
		"email":     user.Email,
		"phone":     user.Phone,
	})
}

func (s *Server) handleUserLogin(ctx *fasthttp.RequestCtx) {
	username := string(ctx.QueryArgs().Peek("username"))
	password := string(ctx.QueryArgs().Peek("password"))
	s.rwm.RLock()
	defer s.rwm.RUnlock()

	user, ok := s.storage.Users[username]
	if !ok {
		writeError(ctx, errors.New("user not exists"))
		return
	}
	if password != user.Password {
		writeError(ctx, errors.New("password is wrong"))
		return
	}
	token := generateToken(username, s.aead)
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("text/plain")
	ctx.WriteString(token)
}

func (s *Server) handleRoomCreate(ctx *fasthttp.RequestCtx) {
	var room struct {
		Name string `json:"name"`
	}
	if !bindJSON(ctx, &room) {
		return
	}
	result, err := s.proposeRaftCommand(ctx, storage.InternalRaftCommand{
		CreateRoom: &storage.CreateRoomCommand{Name: room.Name},
	})
	if err != nil {
		writeError(ctx, err)
		return
	}
	roomID := result.(int)
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("text/plain")
	ctx.WriteString(strconv.Itoa(roomID))
}

func (s *Server) handleRoomQuery(ctx *fasthttp.RequestCtx) {
	idStr := strings.TrimPrefix(string(ctx.Path()), "/room/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(ctx, errors.New("room not exists"))
		return
	}
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	room, ok := s.storage.Rooms[int(id)]
	if !ok {
		writeError(ctx, errors.New("room not exists"))
		return
	}
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("text/plain")
	ctx.WriteString(room.Name)
}

func (s *Server) handleRoomList(ctx *fasthttp.RequestCtx) {
	pageIndex, pageSize, err := parseRequestPage(ctx)
	if err != nil {
		writeError(ctx, err)
		return
	}
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	size := len(s.storage.RoomList)
	start, end := convertPageToRange(size, pageIndex, pageSize)
	type RespRoom struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	respRooms := make([]RespRoom, end-start)
	for i := end - 1; i >= start; i-- {
		room := s.storage.RoomList[i]
		respRooms[end-1-i] = RespRoom{
			Name: room.Name,
			ID:   strconv.Itoa(room.ID),
		}
	}
	writeJSON(ctx, respRooms)
}

func (s *Server) handleRoomListUsers(ctx *fasthttp.RequestCtx) {
	idStr := strings.TrimSuffix(strings.TrimPrefix(string(ctx.Path()), "/room/"), "/users")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(ctx, errors.New("room not exists"))
		return
	}
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	room, ok := s.storage.Rooms[int(id)]
	if !ok {
		writeError(ctx, errors.New("room not exists"))
		return
	}
	users := room.Users
	if users == nil {
		users = make([]string, 0)
	}
	writeJSON(ctx, users)
}

func (s *Server) handleRoomEnter(ctx *fasthttp.RequestCtx) {
	username := ctx.UserValue("username").(string)
	idStr := strings.TrimSuffix(strings.TrimPrefix(string(ctx.Path()), "/room/"), "/enter")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(ctx, errors.New("room not exists"))
		return
	}
	if _, err := s.proposeRaftCommand(ctx, storage.InternalRaftCommand{
		EnterRoom: &storage.EnterRoomCommand{UserName: username, RoomID: int(id)},
	}); err != nil {
		writeError(ctx, err)
	}
}

func (s *Server) handleRoomLeave(ctx *fasthttp.RequestCtx) {
	username := ctx.UserValue("username").(string)
	if _, err := s.proposeRaftCommand(ctx, storage.InternalRaftCommand{
		LeaveRoom: &storage.LeaveRoomCommand{UserName: username},
	}); err != nil {
		writeError(ctx, err)
	}
}

func (s *Server) handleMessageSend(ctx *fasthttp.RequestCtx) {
	var msg struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	if !bindJSON(ctx, &msg) {
		return
	}
	username := ctx.UserValue("username").(string)
	if _, err := s.proposeRaftCommand(ctx, storage.InternalRaftCommand{
		SendMessage: &storage.SendMessageCommand{
			ID:       msg.ID,
			TS:       int(time.Now().Unix()),
			Text:     msg.Text,
			UserName: username,
		},
	}); err != nil {
		writeError(ctx, err)
	}
}

func (s *Server) handleMessageRetrieve(ctx *fasthttp.RequestCtx) {
	pageIndex, pageSize, err := parseRequestPage(ctx)
	if err != nil {
		writeError(ctx, err)
		return
	}
	username := ctx.UserValue("username").(string)
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	roomID := s.storage.Users[username].RoomID
	if roomID <= 0 {
		writeError(ctx, errors.New("user out of room"))
		return
	}
	room, ok := s.storage.Rooms[roomID]
	if !ok {
		writeError(ctx, errors.New("room not exists"))
		return
	}
	size := len(room.Messages)
	start, end := convertPageToRange(size, pageIndex, pageSize)
	type RespMsg struct {
		ID        string `json:"id"`
		Text      string `json:"text"`
		Timestamp string `json:"timestamp"`
	}
	respMsgs := make([]RespMsg, end-start)
	for i := end - 1; i >= start; i-- {
		msg := room.Messages[i]
		respMsgs[end-1-i] = RespMsg{
			ID:        msg.ID,
			Text:      msg.Text,
			Timestamp: strconv.Itoa(msg.TS),
		}
	}
	writeJSON(ctx, respMsgs)
}

func (s *Server) checkAuth(ctx *fasthttp.RequestCtx) bool {
	fields := bytes.Fields(ctx.Request.Header.Peek("Authorization"))
	if len(fields) == 0 {
		writeError(ctx, errors.New("token is missing"))
		return false
	}
	token := fields[len(fields)-1]
	username, ok := parseUserNameFromToken(string(token), s.aead)
	if !ok {
		writeError(ctx, errors.New("token is invalid"))
		return false
	}
	ctx.SetUserValue("username", username)
	return true
}

func (s *Server) newChatHandler() fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		method := strings.ToUpper(string(ctx.Method()))
		path := string(ctx.Path())
		if method == fasthttp.MethodPost && path == "/updateCluster" {
			s.handleClusterUpdate(ctx)
			return
		}

		// The follow requests must be sent after the cluster is started.
		if !s.clusterStarted.Load() {
			writeError(ctx, errors.New("cluster has not started yet"))
			return
		}
		if method == fasthttp.MethodGet && path == "/checkCluster" {
			s.handleClusterCheck(ctx)
			return
		}

		switch {
		// User API.
		case method == fasthttp.MethodPost && path == "/user":
			s.handleUserCreate(ctx)
		case method == fasthttp.MethodGet && strings.HasPrefix(path, "/user/"):
			s.handleUserQuery(ctx)
		case method == fasthttp.MethodGet && path == "/userLogin":
			s.handleUserLogin(ctx)

		// Room API.
		case method == fasthttp.MethodPost && path == "/room" && s.checkAuth(ctx):
			s.handleRoomCreate(ctx)
		case method == fasthttp.MethodGet &&
			strings.HasPrefix(path, "/room/") &&
			strings.Count(path, "/") == 2:
			s.handleRoomQuery(ctx)
		case method == fasthttp.MethodPost && path == "/roomList":
			s.handleRoomList(ctx)
		case method == fasthttp.MethodGet &&
			strings.HasPrefix(path, "/room/") &&
			strings.HasSuffix(path, "/users"):
			s.handleRoomListUsers(ctx)
		case method == fasthttp.MethodPut && strings.HasPrefix(path, "/room/") &&
			strings.HasSuffix(path, "/enter") && s.checkAuth(ctx):
			s.handleRoomEnter(ctx)
		case method == fasthttp.MethodPut && path == "/roomLeave" && s.checkAuth(ctx):
			s.handleRoomLeave(ctx)

		// Message API.
		case method == fasthttp.MethodPost && path == "/message/send" && s.checkAuth(ctx):
			s.handleMessageSend(ctx)
		case method == fasthttp.MethodPost && path == "/message/retrieve" && s.checkAuth(ctx):
			s.handleMessageRetrieve(ctx)
		}
	}
}
