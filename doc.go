// Package minikitex is a tiny RPC runtime for learning how Kitex works.
//
// Kitex (and this toy) is four jobs, not one:
//
//  1. idl     — the contract (service / structs / field ids)
//  2. cmd/gen — turn IDL into typed Go: structs, codec, Client, server adapter
//  3. wire    — the bytes on the network (field-id binary, Thrift-shaped)
//  4. generic — build those same bytes from map/JSON + IDL at runtime
//
// Typed call:  cli.GetUser(ctx, &GetUserReq{ID: 1})
// Generic:     cli.Call(ctx, "GetUser", map[string]any{"id": 1})
//
// The server cannot tell which client you used. That is the whole trick.
package minikitex
