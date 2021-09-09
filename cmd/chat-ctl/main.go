package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var verifyBaseURL func() (string, error)

func printResp(cmd *cobra.Command, resp *http.Response) error {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	cmd.Println(resp.Status)
	cmd.Println(strings.TrimRight(string(data), "\n"))
	return nil
}

func newCmdUserCreate() *cobra.Command {
	var (
		username  string
		firstname string
		lastname  string
		email     string
		password  string
		phone     string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			return printResp(cmd, resp)
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "User's name")
	cmd.Flags().StringVar(&firstname, "firstname", "", "User's firstname")
	cmd.Flags().StringVar(&lastname, "lastname", "", "User's lastname")
	cmd.Flags().StringVar(&email, "email", "", "User's email")
	cmd.Flags().StringVar(&password, "password", "", "User's password")
	cmd.Flags().StringVar(&phone, "phone", "", "User's phone number")
	cmd.MarkFlagRequired("username")
	cmd.MarkFlagRequired("password")
	return cmd
}

func newCmdUserQuery() *cobra.Command {
	var username string
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query information of a user",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			return printResp(cmd, resp)
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "User's name")
	cmd.MarkFlagRequired("username")
	return cmd
}

func newCmdUserLogin() *cobra.Command {
	var (
		username string
		password string
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to chat server",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			reqURL := fmt.Sprintf("%s/userLogin?username=%s&password=%s", baseURL, username, password)
			resp, err := http.Get(reqURL)
			if err != nil {
				return err
			}
			return printResp(cmd, resp)
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "User's name")
	cmd.Flags().StringVar(&password, "password", "", "User's password")
	cmd.MarkFlagRequired("username")
	cmd.MarkFlagRequired("password")
	return cmd
}

func newCmdUser() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users",
	}
	cmd.AddCommand(newCmdUserCreate())
	cmd.AddCommand(newCmdUserQuery())
	cmd.AddCommand(newCmdUserLogin())
	return cmd
}

func newCmdRoomCreate() *cobra.Command {
	var (
		name  string
		token string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new room",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(name) == 0 {
				return errors.New("room name must not be empty")
			}
			baseURL, err := verifyBaseURL()
			if err != nil {
				return nil
			}
			reqURL := baseURL + "/room"
			body := fmt.Sprintf("{\"name\":\"%s\"}", name)
			req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			return printResp(cmd, resp)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "The name of room")
	cmd.Flags().StringVar(&token, "token", "", "User's authenticated token")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("token")
	return cmd
}

func newCmdRoomQuery() *cobra.Command {
	var roomID string
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query information of a room",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			return printResp(cmd, resp)
		},
	}
	cmd.Flags().StringVar(&roomID, "id", "", "The id of room")
	cmd.MarkFlagRequired("id")
	return cmd
}

func newCmdRoomList() *cobra.Command {
	var (
		pageIndex int
		pageSize  int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List rooms",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			return printResp(cmd, resp)
		},
	}
	cmd.Flags().IntVar(&pageIndex, "page-index", 0, "The index of page")
	cmd.Flags().IntVar(&pageSize, "page-size", 10, "The size of per page")
	return cmd
}

func newCmdRoomListUsers() *cobra.Command {
	var roomID string
	cmd := &cobra.Command{
		Use:   "list-users",
		Short: "List all users of a room",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			return printResp(cmd, resp)
		},
	}
	cmd.Flags().StringVar(&roomID, "id", "", "The id of room")
	cmd.MarkFlagRequired("id")
	return cmd
}

func newCmdRoomEnter() *cobra.Command {
	var (
		roomID string
		token  string
	)
	cmd := &cobra.Command{
		Use:   "enter",
		Short: "Enter a room",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			return printResp(cmd, resp)
		},
	}
	cmd.Flags().StringVar(&roomID, "id", "", "The id of room")
	cmd.Flags().StringVar(&token, "token", "", "User's authenticated token")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("token")
	return cmd
}

func newCmdRoomLeave() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "leave",
		Short: "Leave a room",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			return printResp(cmd, resp)
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "User's authenticated token")
	cmd.MarkFlagRequired("token")
	return cmd
}

func newCmdRoom() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "room",
		Short: "Manage rooms and related actions",
	}
	cmd.AddCommand(newCmdRoomCreate())
	cmd.AddCommand(newCmdRoomQuery())
	cmd.AddCommand(newCmdRoomList())
	cmd.AddCommand(newCmdRoomListUsers())
	cmd.AddCommand(newCmdRoomEnter())
	cmd.AddCommand(newCmdRoomLeave())
	return cmd
}

func newCmdMessageSend() *cobra.Command {
	var (
		id    string
		text  string
		token string
	)
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send message to a room",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			return printResp(cmd, resp)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Message id")
	cmd.Flags().StringVar(&text, "text", "", "message text")
	cmd.Flags().StringVar(&token, "token", "", "User's authenticated token")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("text")
	cmd.MarkFlagRequired("token")
	return cmd
}

func newCmdMessageRetrieve() *cobra.Command {
	var (
		pageIndex int
		pageSize  int
		token     string
	)
	cmd := &cobra.Command{
		Use:   "retrieve",
		Short: "Retrieve message from a room",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			return printResp(cmd, resp)
		},
	}
	cmd.Flags().IntVar(&pageIndex, "page-index", 0, "The index of page")
	cmd.Flags().IntVar(&pageSize, "page-size", 10, "The size of per page")
	cmd.Flags().StringVar(&token, "token", "", "User's authenticated token")
	cmd.MarkFlagRequired("token")
	return cmd
}

func newCmdMessage() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Send and retrieve messages",
	}
	cmd.AddCommand(newCmdMessageSend())
	cmd.AddCommand(newCmdMessageRetrieve())
	return cmd
}

func main() {
	var addr string
	verifyBaseURL = func() (string, error) {
		u, err := url.Parse(addr)
		if err != nil {
			return "", err
		}
		if len(u.Scheme) == 0 {
			u.Scheme = "http"
		}
		return u.String(), nil
	}
	cmd := &cobra.Command{
		Use:   "chat-ctl",
		Short: "A simple command line client for chat server",
	}
	cmd.AddCommand(newCmdUser())
	cmd.AddCommand(newCmdRoom())
	cmd.AddCommand(newCmdMessage())
	cmd.PersistentFlags().StringVar(&addr, "addr", "http://127.0.0.1:8080", "Address of server")
	cmd.SetOut(os.Stdout)
	if err := cmd.Execute(); err != nil {
		cmd.Println(err)
		os.Exit(1)
	}
}
