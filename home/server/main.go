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
			{
				Name:  "agent",
				Title: "Agent",
				Desc:  "Agent 控制台 · Go / Python",
				Href:  "/agent/",
			},
			{
				Name:  "pages",
				Title: "pages.lssl.work",
				Desc:  "Agent 页面托管 · put_page 即上线",
				Href:  "https://pages.lssl.work/",
			},
			{
				Name:  "lssl",
				Title: "lssl.work",
				Desc:  "主站入口",
				Href:  "https://lssl.work/",
			},
			{
				Name:  "pingpang",
				Title: "乒乓球小程序",
				Desc:  "微信小程序 · my-pingpang",
				Href:  "https://github.com/cool-sai/my-pingpang",
			},
			{
				Name:  "news",
				Title: "news.lssl.work",
				Desc:  "每日晨报",
				Href:  "https://news.lssl.work/",
			},
			{
				Name:  "board",
				Title: "Board",
				Desc:  "看板 + MCP",
				Href:  "https://lssl.work/board/",
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
