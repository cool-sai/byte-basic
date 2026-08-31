package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"minikitex/gen/order"
)

func main() {
	addr := os.Getenv("ORDER_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8889"
	}
	cli, err := order.NewClient(addr)
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	resp, err := cli.GetOrder(context.Background(), &order.GetOrderReq{ID: 1001})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("GetOrder -> id=%d userId=%d status=%s userName=%s\n",
		resp.ID, resp.UserId, resp.Status, resp.UserName)
}
