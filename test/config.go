package main

type action struct {
	name   string
	cnt    int // when the hit count reaches the limit,the weight will be 0
	weight int // if there are many actions in a same layer,random choose a action to do by weight
}

var ActionTier [][]action
var addrList = []string{"http://127.0.0.1:8080"}

const (
	MessagePageSize = 100
	UserCnt         = 1     // The parameters to initialize the user count.Don't create user in action List
	UseWeightPolicy = false // false is close, true is open. When it's opened,you must fill the weight field
)

func init() {
	ActionTier = [][]action{
		{action{name: "loginUser", cnt: 10}},
		{action{name: "createRoom", cnt: 1}},
		{action{name: "enterRoom", cnt: 1}},
		{action{name: "queryRoom", cnt: 1}},
		{action{name: "sendMessage", cnt: 5}, action{name: "RetrieveMessage", cnt: 5}},
		{action{name: "leaveRoom", cnt: 1}},
	}
}
