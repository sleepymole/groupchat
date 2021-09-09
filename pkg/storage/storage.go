package storage

import (
	"bytes"
	"encoding/gob"
	"sort"
)

type User struct {
	UserName  string
	FirstName string
	LastName  string
	Email     string
	Password  string
	Phone     string
	RoomID    int
}

type Message struct {
	ID   string
	TS   int
	Text string
}

type Room struct {
	ID       int
	Name     string
	Users    []string
	Messages []*Message
}

type Snapshot struct {
	Index     uint64
	Users     map[string]*User
	Rooms     map[int]*Room
	SecretKey []byte
}

type Storage struct {
	Snapshot
	NextRoomID int
	RoomList   []*Room
}

func NewStorage() *Storage {
	return &Storage{
		Snapshot: Snapshot{
			Users: make(map[string]*User),
			Rooms: make(map[int]*Room),
		},
		NextRoomID: 1,
	}
}

func (s *Storage) GenSnapshot() []byte {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&s.Snapshot); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (s *Storage) RecoverFromSnapshot(snapshot []byte) {
	if err := gob.NewDecoder(bytes.NewReader(snapshot)).Decode(&s.Snapshot); err != nil {
		panic(err)
	}
	if s.Users == nil {
		s.Users = make(map[string]*User)
	}
	if s.Rooms == nil {
		s.Rooms = make(map[int]*Room)
	}
	for _, room := range s.Rooms {
		if room.ID >= s.NextRoomID {
			s.NextRoomID = room.ID + 1
		}
		s.RoomList = append(s.RoomList, room)
	}
	sort.Slice(s.RoomList, func(i, j int) bool {
		return s.RoomList[i].ID < s.RoomList[j].ID
	})
}
