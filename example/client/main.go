package main

import (
	"context"
	"fmt"
	"log"

	"minikitex/gen/user"
)

func main() {
	cli, err := user.NewClient("127.0.0.1:8888")
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	resp, err := cli.GetUser(context.Background(), &user.GetUserReq{ID: 1})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("typed GetUser -> id=%d name=%s\n", resp.ID, resp.Name)

	pong, err := cli.Ping(context.Background(), &user.PingReq{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("typed Ping    -> %s\n", pong.Msg)
}
