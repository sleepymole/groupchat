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

	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"

	"github.com/gozssky/groupchat/pkg/raftnode"
	"github.com/gozssky/groupchat/pkg/storage"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func writeError(c *gin.Context, err error) {
	c.Data(http.StatusBadRequest, "text/plain", []byte(fmt.Sprintf("Error: %v", err)))
}

func writeJSON(c *gin.Context, obj interface{}) {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	c.Data(http.StatusOK, "application/json", data)
}

func shouldBindJSON(c *gin.Context, obj interface{}) error {
	data, err := c.GetRawData()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, obj)
}

func (s *Server) handleClusterUpdate(c *gin.Context) {
	var clusterIPs []string
	if err := shouldBindJSON(c, &clusterIPs); err != nil {
		writeError(c, err)
		return
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		writeError(c, err)
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
		writeError(c, errors.New("local ip not exists in cluster"))
		return
	}
	//goland:noinspection HttpUrlsUsage
	localURL := fmt.Sprintf("http://%s:%d", localIP, s.port)
	var remoteURLs []string
	for _, ip := range clusterIPs {
		if ip == localIP {
			continue
		}
		//goland:noinspection HttpUrlsUsage
		remoteURL := fmt.Sprintf("http://%s:%d", ip, s.port)
		remoteURLs = append(remoteURLs, remoteURL)
		if len(c.GetHeader("Referer")) == 0 {
			go func() {
				data, err := json.Marshal(clusterIPs)
				if err != nil {
					panic(err)
				}
				req, err := http.NewRequest(http.MethodPost, remoteURL+"/updateCluster", bytes.NewReader(data))
				if err != nil {
					panic(err)
				}
				req.Header.Add("Referer", localURL)
				logger := s.lg.With(zap.String("remote-url", remoteURL))
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

func (s *Server) handleClusterCheck(_ *gin.Context) {}

func (s *Server) handleUserCreate(c *gin.Context) {
	var user struct {
		UserName  string `json:"username"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		Phone     string `json:"phone"`
	}
	if err := shouldBindJSON(c, &user); err != nil {
		writeError(c, err)
		return
	}
	if _, err := s.proposeRaftCommand(c.Request.Context(), storage.InternalRaftCommand{
		CreateUser: &storage.CreateUserCommand{
			UserName:  user.UserName,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Password:  user.Password,
			Phone:     user.Phone,
		},
	}); err != nil {
		writeError(c, err)
		return
	}
}

func (s *Server) handleUserQuery(c *gin.Context) {
	name := c.Param("name")
	s.rwm.RLock()
	defer s.rwm.RUnlock()

	user, ok := s.storage.Users[name]
	if !ok {
		writeError(c, errors.New("user not exists"))
		return
	}
	writeJSON(c, gin.H{
		"firstName": user.FirstName,
		"lastName":  user.LastName,
		"email":     user.Email,
		"phone":     user.Phone,
	})
}

func (s *Server) handleUserLogin(c *gin.Context) {
	username := c.Query("username")
	password := c.Query("password")
	s.rwm.RLock()
	defer s.rwm.RUnlock()

	user, ok := s.storage.Users[username]
	if !ok {
		writeError(c, errors.New("user not exists"))
		return
	}
	if password != user.Password {
		writeError(c, errors.New("password is wrong"))
		return
	}
	token := generateToken(username, s.aead)
	c.Data(http.StatusOK, "text/plain", []byte(token))
}

func (s *Server) handleRoomCreate(c *gin.Context) {
	var room struct {
		Name string `json:"name"`
	}
	if err := shouldBindJSON(c, &room); err != nil {
		writeError(c, err)
		return
	}
	result, err := s.proposeRaftCommand(c.Request.Context(), storage.InternalRaftCommand{
		CreateRoom: &storage.CreateRoomCommand{Name: room.Name},
	})
	if err != nil {
		writeError(c, err)
		return
	}
	roomID := result.(int)
	c.Data(http.StatusOK, "text/plain", []byte(strconv.Itoa(roomID)))
}

func (s *Server) handleRoomQuery(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(c, errors.New("room not exists"))
		return
	}
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	room, ok := s.storage.Rooms[int(id)]
	if !ok {
		writeError(c, errors.New("room not exists"))
		return
	}
	c.Data(http.StatusOK, "text/plain", []byte(room.Name))
}

func (s *Server) handleRoomList(c *gin.Context) {
	pageIndex, pageSize, err := parseRequestPage(c)
	if err != nil {
		writeError(c, err)
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
	writeJSON(c, respRooms)
}

func (s *Server) handleRoomListUsers(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(c, errors.New("room not exists"))
		return
	}
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	room, ok := s.storage.Rooms[int(id)]
	if !ok {
		writeError(c, errors.New("room not exists"))
		return
	}
	users := room.Users
	if users == nil {
		users = make([]string, 0)
	}
	writeJSON(c, users)
}

func (s *Server) handleRoomEnter(c *gin.Context) {
	username, _ := c.Get("username")
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(c, errors.New("room not exists"))
		return
	}
	if _, err := s.proposeRaftCommand(c.Request.Context(), storage.InternalRaftCommand{
		EnterRoom: &storage.EnterRoomCommand{UserName: username.(string), RoomID: int(id)},
	}); err != nil {
		writeError(c, err)
	}
}

func (s *Server) handleRoomLeave(c *gin.Context) {
	username, _ := c.Get("username")
	if _, err := s.proposeRaftCommand(c.Request.Context(), storage.InternalRaftCommand{
		LeaveRoom: &storage.LeaveRoomCommand{UserName: username.(string)},
	}); err != nil {
		writeError(c, err)
	}
}

func (s *Server) handleMessageSend(c *gin.Context) {
	var msg struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	if err := shouldBindJSON(c, &msg); err != nil {
		writeError(c, err)
		return
	}
	username, _ := c.Get("username")
	if _, err := s.proposeRaftCommand(c.Request.Context(), storage.InternalRaftCommand{
		SendMessage: &storage.SendMessageCommand{
			ID:       msg.ID,
			TS:       int(time.Now().Unix()),
			Text:     msg.Text,
			UserName: username.(string),
		},
	}); err != nil {
		writeError(c, err)
	}
}

func (s *Server) handleMessageRetrieve(c *gin.Context) {
	pageIndex, pageSize, err := parseRequestPage(c)
	if err != nil {
		writeError(c, err)
		return
	}
	username, _ := c.Get("username")
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	roomID := s.storage.Users[username.(string)].RoomID
	if roomID <= 0 {
		writeError(c, errors.New("user out of room"))
		return
	}
	room, ok := s.storage.Rooms[roomID]
	if !ok {
		writeError(c, errors.New("room not exists"))
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
	writeJSON(c, respMsgs)
}

func (s *Server) authRequired(c *gin.Context) {
	fields := strings.Fields(c.GetHeader("Authorization"))
	if len(fields) == 0 {
		writeError(c, errors.New("token is missing"))
		c.Abort()
		return
	}
	token := fields[len(fields)-1]
	if username, ok := parseUserNameFromToken(token, s.aead); ok {
		c.Set("username", username)
	} else {
		writeError(c, errors.New("token is invalid"))
		c.Abort()
	}
}

func (s *Server) clusterStartedRequired(c *gin.Context) {
	if !s.clusterStarted.Load() {
		writeError(c, errors.New("cluster has not started yet"))
		c.Abort()
	}
}

func (s *Server) newChatRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.POST("/updateCluster", s.handleClusterUpdate)

	// The follow requests must be sent after the cluster is started.
	router.Use(s.clusterStartedRequired)

	router.GET("/checkCluster", s.handleClusterCheck)

	// User API.
	router.POST("/user", s.handleUserCreate)
	router.GET("/user/:name", s.handleUserQuery)
	router.GET("/userLogin", s.handleUserLogin)

	// Room API.
	router.POST("/room", s.authRequired, s.handleRoomCreate)
	router.GET("/room/:id", s.handleRoomQuery)
	router.POST("/roomList", s.handleRoomList)
	router.GET("/room/:id/users", s.handleRoomListUsers)
	router.PUT("/room/:id/enter", s.authRequired, s.handleRoomEnter)
	router.PUT("/roomLeave", s.authRequired, s.handleRoomLeave)

	// Message API.
	router.POST("/message/send", s.authRequired, s.handleMessageSend)
	router.POST("/message/retrieve", s.authRequired, s.handleMessageRetrieve)

	return router
}
