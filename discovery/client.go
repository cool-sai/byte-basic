package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var httpc = &http.Client{Timeout: 2 * time.Second}

// Register puts a Consul catalog entry and immediately passes its TTL check.
func Register(base, name, addr string) error {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return err
	}
	id := host
	body, _ := json.Marshal(map[string]any{
		"ID":      id,
		"Name":    name,
		"Address": host,
		"Port":    port,
		"Check": map[string]any{
			"TTL":                            "8s",
			"DeregisterCriticalServiceAfter": "1m",
		},
	})
	if err := put(base+"/v1/agent/service/register", body); err != nil {
		return err
	}
	return put(base+"/v1/agent/check/pass/service:"+url.PathEscape(id), nil)
}

func Lookup(base, name string) ([]string, error) {
	u := strings.TrimRight(base, "/") + "/v1/health/service/" + url.PathEscape(name) + "?passing=true"
	resp, err := httpc.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("lookup %s", resp.Status)
	}
	var rows []struct {
		Service struct {
			Address string `json:"Address"`
			Port    int    `json:"Port"`
		} `json:"Service"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	var addrs []string
	for _, row := range rows {
		if row.Service.Address == "" || row.Service.Port == 0 {
			continue
		}
		addrs = append(addrs, net.JoinHostPort(row.Service.Address, strconv.Itoa(row.Service.Port)))
	}
	sort.Strings(addrs)
	return addrs, nil
}

func put(url string, body []byte) error {
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("consul %s %s", url, resp.Status)
	}
	return nil
}
