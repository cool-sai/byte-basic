package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var httpc = &http.Client{Timeout: 2 * time.Second}

func Register(base, name, addr string) error {
	b, _ := json.Marshal(Instance{Name: name, Addr: addr})
	resp, err := httpc.Post(base+"/register", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("register %s", resp.Status)
	}
	return nil
}

func Lookup(base, name string) ([]string, error) {
	resp, err := httpc.Get(base + "/lookup?name=" + name)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("lookup %s", resp.Status)
	}
	var out struct {
		Addrs []string `json:"addrs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Addrs, nil
}
