package main

import (
	"errors"
	"log"
	"math/rand"
	"sync"
	"time"
)

type testUser struct {
	username string
	password string
	token    string
	roomID   string
	isLogin  bool
	isEnter  bool
}

type RoomCache struct {
	allRoom []string
	roomRW  *sync.RWMutex
}

func (r *RoomCache) getRoomsCnt() int {
	r.roomRW.RLock()
	defer r.roomRW.Unlock()
	return len(r.allRoom)
}

func (r *RoomCache) pickOneRoomRandom() string {
	r.roomRW.RLock()
	defer r.roomRW.RUnlock()
	length := len(r.allRoom)
	if length == 0 {
		return ""
	}
	return r.allRoom[genRandInt(length)]
}
func (r *RoomCache) addRoom(roomID string) {
	r.roomRW.Lock()
	defer r.roomRW.Unlock()
	r.allRoom = append(r.allRoom, roomID)
}

type UserCache struct {
	allUsers []*testUser
	userRW   *sync.RWMutex
}

func (u *UserCache) addUser(v *testUser) {
	u.userRW.Lock()
	defer u.userRW.Unlock()
	u.allUsers = append(u.allUsers, v)
}

func (u *UserCache) getUsersCnt() int {
	u.userRW.RLock()
	defer u.userRW.RUnlock()
	return len(u.allUsers)
}

func (u *UserCache) pickOneUserRandom() *testUser {
	u.userRW.RLock()
	defer u.userRW.RUnlock()
	length := len(u.allUsers)
	if length == 0 {
		return nil
	}
	index := genRandInt(length)
	return u.allUsers[index]
}

var roomCache *RoomCache
var userCache *UserCache

func initCache() {
	roomCache = &RoomCache{
		allRoom: make([]string, 0),
		roomRW:  new(sync.RWMutex),
	}
	userCache = &UserCache{
		allUsers: make([]*testUser, 0),
		userRW:   new(sync.RWMutex),
	}
}
func genRandString(n int) string {
	rand.Seed(time.Now().UnixNano())
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
func genRandInt(n int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(n)
}

func main() {
	initCache()
	var wg sync.WaitGroup
	if UserCnt == 0 {
		log.Fatal("the UserCnt can't be 0!")
	}
	wg.Add(UserCnt)
	for i := 0; i < UserCnt; i++ {
		if err, user := createUserWrapper(); err != nil {
			log.Printf("create user failed")
			wg.Done()
		} else {
			userCache.addUser(user)
			go func() {
				defer wg.Done()
				actionTier := make([][]action, len(ActionTier))
				for index := range ActionTier {
					actionTier[index] = make([]action, len(ActionTier[index]))
					copy(actionTier[index], ActionTier[index])
				}
				userLife(user, actionTier)
			}()
		}
	}
	wg.Wait()

}

func userLife(nowUser *testUser, actionTier [][]action) {
	for _, tier := range actionTier {
		for {
			if ok, v := pickOneAction(tier); ok {
				v.cnt--
				log.Printf("name:%s cnt:%d\n", v.name, v.cnt)
				if err := doAction(*v, nowUser); err != nil {
					log.Printf("%+v do %s error,error is %s", nowUser, v.name, err.Error())
				}
			} else {
				break
			}
		}
	}
	log.Println("finish task")
}

func createUserWrapper() (error, *testUser) {
	text := genRandString(15)
	if err := createUser(text, text, text, text, text, text); err != nil {
		return err, nil
	}
	return nil, &testUser{username: text, password: text, token: "", roomID: "", isLogin: false, isEnter: false}
}

func doAction(v action, nowUser *testUser) error {
	if v.name == "queryUser" {
		if err := queryUser(userCache.pickOneUserRandom().username); err != nil {
			return err
		}
	} else if v.name == "loginUser" {
		if !nowUser.isLogin {
			if err, code := loginUser(nowUser.username, nowUser.password); err != nil {
				return err
			} else {
				nowUser.token = code
				nowUser.isLogin = true
			}
		}
	} else if v.name == "createRoom" {
		if nowUser.isLogin {
			roomName := genRandString(15)
			if err, code := createRoom(roomName, nowUser.token); err != nil {
				return err
			} else {
				nowUser.roomID = code
				roomCache.addRoom(code)
			}
		}
	} else if v.name == "queryRoom" {
		roomID := roomCache.pickOneRoomRandom()
		if len(roomID) == 0 {
			return errors.New("no rooms")
		}
		if err := queryRoom(roomID); err != nil {
			return err
		}
	} else if v.name == "listRoom" {
		if err := listRoom(0, roomCache.getRoomsCnt()); err != nil {
			return err
		}
	} else if v.name == "listUsers" {
		if err := listUsers(roomCache.pickOneRoomRandom()); err != nil {
			return err
		}
	} else if v.name == "enterRoom" {
		roomID := roomCache.pickOneRoomRandom()
		if len(roomID) == 0 {
			return errors.New("no rooms")
		}
		if err := enterRoom(roomID, nowUser.token); err != nil {
			return err
		} else {
			nowUser.isEnter = true
		}
	} else if v.name == "leaveRoom" {
		if err := leaveRoom(nowUser.token); err != nil {
			return err
		} else {
			nowUser.isEnter = false
		}
	} else if v.name == "sendMessage" {
		messageID := genRandString(15)
		text := messageID
		if err := sendMessage(messageID, text, nowUser.token); err != nil {
			return err
		}
	} else if v.name == "RetrieveMessage" {
		if err := RetrieveMessage(0, MessagePageSize, nowUser.token); err != nil {
			return err
		}
	}
	return nil
}
