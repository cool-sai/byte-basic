package main

import (
	"context"
	"fmt"
	"log"

	"minikitex/generic"
	"minikitex/idl"
)

func main() {
	spec, err := idl.ParseFile("idl/user.thrift")
	if err != nil {
		log.Fatal(err)
	}
	cli, err := generic.Dial("127.0.0.1:8888", spec)
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()
	ctx := context.Background()

	// Map generic — Kitex MapThriftGeneric
	resp, err := cli.Call(ctx, "GetUser", map[string]any{"id": 1})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("map  GetUser -> %v\n", resp)

	// JSON generic — Kitex JSONThriftGeneric
	resp, err = cli.Call(ctx, "GetUser", `{"id": 2}`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("json GetUser -> %v\n", resp)

	resp, err = cli.Call(ctx, "Ping", map[string]any{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("map  Ping    -> %v\n", resp)
}
