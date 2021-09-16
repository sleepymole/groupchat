package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func verifyBaseURL() (string, error) {
	index := genRandInt(len(addrList))
	u, err := url.Parse(addrList[index])
	if err != nil {
		return "", err
	}
	if len(u.Scheme) == 0 {
		u.Scheme = "http"
	}
	return u.String(), nil
}
func printResp(resp *http.Response) error {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Println(resp.Status)
	fmt.Println(strings.TrimRight(string(data), "\n"))
	return nil
}

func getResp(resp *http.Response) (error, string) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err, ""
	}
	result := strings.TrimRight(string(data), "\n")
	fmt.Println(resp.Status)
	fmt.Println(result)
	return nil, result
}

func createUser(username, password, firstname, lastname, email, phone string) error {
	if len(username) == 0 {
		return errors.New("username must not be empty")
	}
	if len(password) == 0 {
		return errors.New("password must not be empty")
	}
	baseURL, err := verifyBaseURL()
	if err != nil {
		return nil
	}
	infos := map[string]string{
		"username":  username,
		"firstName": firstname,
		"lastName":  lastname,
		"email":     email,
		"password":  password,
		"phone":     phone,
	}
	data, err := json.Marshal(&infos)
	if err != nil {
		return err
	}
	reqURL := baseURL + "/user"
	resp, err := http.Post(reqURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	return printResp(resp)
}

func queryUser(username string) error {
	if len(username) == 0 {
		return errors.New("username must not be empty")
	}
	baseURL, err := verifyBaseURL()
	if err != nil {
		return nil
	}
	reqURL := fmt.Sprintf("%s/user/%s", baseURL, username)
	resp, err := http.Get(reqURL)
	if err != nil {
		return err
	}
	return printResp(resp)
}

func loginUser(username string, password string) (error, string) {
	if len(username) == 0 {
		return errors.New("username must not be empty"), ""
	}
	if len(password) == 0 {
		return errors.New("password must not be empty"), ""
	}
	baseURL, err := verifyBaseURL()
	if err != nil {
		return nil, ""
	}
	reqURL := fmt.Sprintf("%s/userLogin?username=%s&password=%s", baseURL, username, password)
	resp, err := http.Get(reqURL)
	if err != nil {
		return err, ""
	}
	return getResp(resp)
}

func createRoom(name string, token string) (error, string) {
	if len(name) == 0 {
		return errors.New("room name must not be empty"), ""
	}
	baseURL, err := verifyBaseURL()
	if err != nil {
		return nil, ""
	}
	reqURL := baseURL + "/room"
	body := fmt.Sprintf("{\"name\":\"%s\"}", name)
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(body))
	if err != nil {
		return err, ""
	}
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err, ""
	}
	return getResp(resp)
}

func queryRoom(roomID string) error {
	if len(roomID) == 0 {
		return errors.New("room id must not be empty")
	}
	baseURL, err := verifyBaseURL()
	if err != nil {
		return nil
	}
	reqURL := fmt.Sprintf("%s/room/%s", baseURL, roomID)
	resp, err := http.Get(reqURL)
	if err != nil {
		return err
	}
	return printResp(resp)
}

func listRoom(pageIndex, pageSize int) error {
	baseURL, err := verifyBaseURL()
	if err != nil {
		return nil
	}
	reqURL := baseURL + "/roomList"
	body := fmt.Sprintf("{\"pageIndex\":%d,\"pageSize\":%d}", pageIndex, pageSize)
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return printResp(resp)
}

func listUsers(roomID string) error {
	if len(roomID) == 0 {
		return errors.New("room id must not be empty")
	}
	baseURL, err := verifyBaseURL()
	if err != nil {
		return nil
	}
	reqURL := fmt.Sprintf("%s/room/%s/users", baseURL, roomID)
	resp, err := http.Get(reqURL)
	if err != nil {
		return err
	}
	return printResp(resp)
}

func enterRoom(roomID, token string) error {
	if len(roomID) == 0 {
		return errors.New("room id must not be empty")
	}
	baseURL, err := verifyBaseURL()
	if err != nil {
		return nil
	}
	reqURL := fmt.Sprintf("%s/room/%s/enter", baseURL, roomID)
	req, err := http.NewRequest(http.MethodPut, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return printResp(resp)
}

func leaveRoom(token string) error {
	baseURL, err := verifyBaseURL()
	if err != nil {
		return nil
	}
	reqURL := baseURL + "/roomLeave"
	req, err := http.NewRequest(http.MethodPut, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return printResp(resp)
}

func sendMessage(id, text, token string) error {
	baseURL, err := verifyBaseURL()
	if err != nil {
		return nil
	}
	reqURL := baseURL + "/message/send"
	body := fmt.Sprintf("{\"id\":\"%s\",\"text\":\"%s\"}", id, text)
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return printResp(resp)
}

func RetrieveMessage(pageIndex, pageSize int, token string) error {
	baseURL, err := verifyBaseURL()
	if err != nil {
		return nil
	}
	reqURL := baseURL + "/message/retrieve"
	body := fmt.Sprintf("{\"pageIndex\":%d,\"pageSize\":%d}", pageIndex, pageSize)
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return printResp(resp)
}
