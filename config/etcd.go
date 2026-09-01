package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

var (
	httpc  = &http.Client{Timeout: 2 * time.Second}
	watchc = &http.Client{} // watch is a long stream
)

type Var struct {
	fallback string
	v        atomic.Value // string
}

func NewVar(fallback string) *Var {
	x := &Var{fallback: fallback}
	x.v.Store(fallback)
	return x
}

func (x *Var) Get() string {
	s, _ := x.v.Load().(string)
	return s
}

func (x *Var) set(s string) {
	old, _ := x.v.Load().(string)
	if old == s {
		return
	}
	x.v.Store(s)
	log.Printf("config %q -> %q", old, s)
}

// Watch GET then long-poll. Missing key keeps fallback. etcd down keeps last value.
func (x *Var) Watch(base, key string) {
	for {
		if err := x.sync(base, key); err != nil {
			log.Println("config:", err)
			time.Sleep(time.Second)
		}
	}
}

func (x *Var) sync(base, key string) error {
	val, ok, rev, err := Get(base, key)
	if err != nil {
		return err
	}
	if ok {
		x.set(val)
	}
	return watch(base, key, rev+1, func(val string, del bool) {
		if del {
			x.set(x.fallback)
			return
		}
		x.set(val)
	})
}

func Get(base, key string) (val string, ok bool, rev int64, err error) {
	body, _ := json.Marshal(map[string]string{"key": b64(key)})
	raw, err := post(httpc, strings.TrimRight(base, "/")+"/v3/kv/range", body)
	if err != nil {
		return "", false, 0, err
	}
	var resp rangeResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", false, 0, err
	}
	rev, _ = resp.Header.Revision.Int64()
	if len(resp.Kvs) == 0 {
		return "", false, rev, nil
	}
	val, err = unb64(resp.Kvs[0].Value)
	return val, err == nil, rev, err
}

func Put(base, key, val string) error {
	body, _ := json.Marshal(map[string]string{"key": b64(key), "value": b64(val)})
	_, err := post(httpc, strings.TrimRight(base, "/")+"/v3/kv/put", body)
	return err
}

type KV struct {
	Key, Value string
}

func List(base string) ([]KV, error) {
	body, _ := json.Marshal(map[string]string{
		"key":       b64("\x00"),
		"range_end": b64("\x00"),
	})
	raw, err := post(httpc, strings.TrimRight(base, "/")+"/v3/kv/range", body)
	if err != nil {
		return nil, err
	}
	var resp rangeResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	var out []KV
	for _, kv := range resp.Kvs {
		k, err1 := unb64(kv.Key)
		v, err2 := unb64(kv.Value)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, KV{Key: k, Value: v})
	}
	return out, nil
}

func Delete(base, key string) error {
	body, _ := json.Marshal(map[string]string{"key": b64(key)})
	_, err := post(httpc, strings.TrimRight(base, "/")+"/v3/kv/deleterange", body)
	return err
}

func watch(base, key string, start int64, fn func(val string, del bool)) error {
	reqBody, _ := json.Marshal(map[string]any{
		"create_request": map[string]any{
			"key":            b64(key),
			"start_revision": start,
		},
	})
	resp, err := watchc.Post(strings.TrimRight(base, "/")+"/v3/watch", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("watch %s %s", resp.Status, b)
	}
	dec := json.NewDecoder(resp.Body)
	for {
		var msg watchMsg
		if err := dec.Decode(&msg); err != nil {
			return err
		}
		for _, ev := range msg.Result.Events {
			if ev.Type == "DELETE" {
				fn("", true)
				continue
			}
			val, err := unb64(ev.Kv.Value)
			if err != nil {
				continue
			}
			fn(val, false)
		}
	}
}

func post(c *http.Client, url string, body []byte) ([]byte, error) {
	resp, err := c.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s", resp.Status, b)
	}
	return b, nil
}

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func unb64(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type rangeResp struct {
	Header struct {
		Revision json.Number `json:"revision"`
	} `json:"header"`
	Kvs []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"kvs"`
}

type watchMsg struct {
	Result struct {
		Events []struct {
			Type string `json:"type"`
			Kv   struct {
				Value string `json:"value"`
			} `json:"kv"`
		} `json:"events"`
	} `json:"result"`
}
