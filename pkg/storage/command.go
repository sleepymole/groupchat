package storage

import (
	"bytes"
	"encoding/gob"
	"errors"
)

var (
	ErrUserNotExists     = errors.New("user not exists")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrRoomNotExists     = errors.New("room not exists")
	ErrUserOutOfRoom     = errors.New("user out of room")
)

type Command interface {
	Execute(s *Storage) *ExecuteResult
}

type ExecuteResult struct {
	Result interface{}
	Err    error
}

type InitSecretKeyCommand struct {
	SecretKey []byte
}

func (c *InitSecretKeyCommand) Execute(s *Storage) *ExecuteResult {
	if len(s.SecretKey) == 0 {
		s.SecretKey = append(s.SecretKey, c.SecretKey...)
	}
	return &ExecuteResult{Result: append([]byte(nil), s.SecretKey...)}
}

type CreateUserCommand struct {
	UserName  string
	FirstName string
	LastName  string
	Email     string
	Password  string
	Phone     string
}

func (c *CreateUserCommand) Execute(s *Storage) *ExecuteResult {
	if _, ok := s.Users[c.UserName]; ok {
		return &ExecuteResult{Err: ErrUserAlreadyExists}
	}
	s.Users[c.UserName] = &User{
		UserName:  c.UserName,
		FirstName: c.FirstName,
		LastName:  c.LastName,
		Email:     c.Email,
		Password:  c.Password,
		Phone:     c.Phone,
		RoomID:    0,
	}
	return &ExecuteResult{}
}

type CreateRoomCommand struct {
	Name string
}

func (c *CreateRoomCommand) Execute(s *Storage) *ExecuteResult {
	room := &Room{
		ID:   s.NextRoomID,
		Name: c.Name,
	}
	s.NextRoomID++
	s.Rooms[room.ID] = room
	s.RoomList = append(s.RoomList, room)
	return &ExecuteResult{Result: room.ID}
}

func removeUser(users []string, user string) []string {
	j := 0
	for i := 0; i < len(users); i++ {
		if users[i] != user {
			users[j] = users[i]
			j += 1
		}
	}
	return users[:j]
}

type EnterRoomCommand struct {
	UserName string
	RoomID   int
}

func (c *EnterRoomCommand) Execute(s *Storage) *ExecuteResult {
	user, ok := s.Users[c.UserName]
	if !ok {
		return &ExecuteResult{Err: ErrUserNotExists}
	}
	if _, ok := s.Rooms[c.RoomID]; !ok {
		return &ExecuteResult{Err: ErrRoomNotExists}
	}
	if user.RoomID > 0 {
		if user.RoomID == c.RoomID {
			return &ExecuteResult{}
		}
		room := s.Rooms[user.RoomID]
		room.Users = removeUser(room.Users, user.UserName)
	}
	user.RoomID = c.RoomID
	room := s.Rooms[user.RoomID]
	room.Users = append(room.Users, user.UserName)
	return &ExecuteResult{}
}

type LeaveRoomCommand struct {
	UserName string
}

func (c *LeaveRoomCommand) Execute(s *Storage) *ExecuteResult {
	user, ok := s.Users[c.UserName]
	if !ok {
		return &ExecuteResult{Err: ErrUserNotExists}
	}
	if user.RoomID <= 0 {
		return &ExecuteResult{}
	}
	room := s.Rooms[user.RoomID]
	room.Users = removeUser(room.Users, user.UserName)
	user.RoomID = 0
	return &ExecuteResult{}
}

type SendMessageCommand struct {
	ID       string
	TS       int
	Text     string
	UserName string
}

func (c *SendMessageCommand) Execute(s *Storage) *ExecuteResult {
	user, ok := s.Users[c.UserName]
	if !ok {
		return &ExecuteResult{Err: ErrUserNotExists}
	}
	if user.RoomID <= 0 {
		return &ExecuteResult{Err: ErrUserOutOfRoom}
	}
	room, ok := s.Rooms[user.RoomID]
	if !ok {
		return &ExecuteResult{Err: ErrRoomNotExists}
	}
	room.Messages = append(room.Messages, &Message{
		ID:   c.ID,
		TS:   c.TS,
		Text: c.Text,
	})
	return &ExecuteResult{}
}

type InternalRaftCommand struct {
	ID            uint64
	InitSecretKey *InitSecretKeyCommand
	CreateUser    *CreateUserCommand
	CreateRoom    *CreateRoomCommand
	EnterRoom     *EnterRoomCommand
	LeaveRoom     *LeaveRoomCommand
	SendMessage   *SendMessageCommand
}

func (c *InternalRaftCommand) Execute(s *Storage) *ExecuteResult {
	result := &ExecuteResult{}
	switch {
	case c.InitSecretKey != nil:
		result = c.InitSecretKey.Execute(s)
	case c.CreateUser != nil:
		result = c.CreateUser.Execute(s)
	case c.CreateRoom != nil:
		result = c.CreateRoom.Execute(s)
	case c.EnterRoom != nil:
		result = c.EnterRoom.Execute(s)
	case c.LeaveRoom != nil:
		result = c.LeaveRoom.Execute(s)
	case c.SendMessage != nil:
		result = c.SendMessage.Execute(s)
	}
	return result
}

func (c *InternalRaftCommand) MustMarshalGOB() []byte {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(c); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (c *InternalRaftCommand) MustUnmarshalGOB(data []byte) {
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(c); err != nil {
		panic(err)
	}
}
