package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	v1 "minikitex/gen/home/v1"
	"minikitex/gen/home/v1/homev1connect"
)

func TestListSitesREST(t *testing.T) {
	h, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/sites", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Sites []struct {
			Href string `json:"href"`
		} `json:"sites"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Sites) == 0 || body.Sites[0].Href != "/minikitex/" {
		t.Fatalf("%s", rec.Body.String())
	}
}

func TestListSitesConnect(t *testing.T) {
	h, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	cli := homev1connect.NewHomeServiceClient(http.DefaultClient, srv.URL)
	res, err := cli.ListSites(context.Background(), connect.NewRequest(&v1.ListSitesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Msg.Sites) == 0 || res.Msg.Sites[0].Href != "/minikitex/" {
		t.Fatalf("%+v", res.Msg)
	}
}
