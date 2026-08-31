package runtime

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

type Client struct {
	conn net.Conn
	mu   sync.Mutex
	seq  uint32
}

func Dial(addr string) (*Client, error) {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &Client{conn: c}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) Call(ctx context.Context, method string, body []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.seq++
	seq := c.seq
	if dl, ok := ctx.Deadline(); ok {
		_ = c.conn.SetDeadline(dl)
		defer c.conn.SetDeadline(time.Time{})
	}

	if err := writeMsg(c.conn, MsgCall, seq, method, body); err != nil {
		return nil, err
	}
	msg, err := readMsg(c.conn)
	if err != nil {
		return nil, err
	}
	if msg.seq != seq {
		return nil, fmt.Errorf("runtime: seq mismatch")
	}
	if msg.typ == MsgException {
		return nil, fmt.Errorf("%s", msg.body)
	}
	if msg.typ != MsgReply {
		return nil, fmt.Errorf("runtime: bad msg type %d", msg.typ)
	}
	return msg.body, nil
}
