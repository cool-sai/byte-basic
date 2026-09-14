package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"connectrpc.com/vanguard"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	v1 "minikitex/gen/home/v1"
	"minikitex/gen/home/v1/homev1connect"
)

type server struct{}

func (server) ListSites(context.Context, *connect.Request[v1.ListSitesRequest]) (*connect.Response[v1.ListSitesResponse], error) {
	return connect.NewResponse(&v1.ListSitesResponse{
		Sites: []*v1.Site{
			{
				Name:  "minikitex",
				Title: "minikitex",
				Desc:  "控制台 · SCM / BAM / AGW / TLB / Deploy",
				Href:  "/minikitex/",
			},
		},
	}), nil
}

func newHandler() (http.Handler, error) {
	path, h := homev1connect.NewHomeServiceHandler(server{})
	transcoder, err := vanguard.NewTranscoder([]*vanguard.Service{
		vanguard.NewService(path, h),
	})
	if err != nil {
		return nil, err
	}
	return h2c.NewHandler(transcoder, &http2.Server{}), nil
}

func main() {
	addr := os.Getenv("LISTEN")
	if addr == "" {
		addr = "0.0.0.0:8082"
	}
	h, err := newHandler()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("home", addr)
	log.Fatal(http.ListenAndServe(addr, h))
}
