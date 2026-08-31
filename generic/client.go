package generic

import (
	"context"
	"fmt"

	"minikitex/idl"
	"minikitex/runtime"
)

type Client struct {
	inner *runtime.Client
	spec  *idl.Spec
}

func Dial(addr string, spec *idl.Spec) (*Client, error) {
	c, err := runtime.Dial(addr)
	if err != nil {
		return nil, err
	}
	return &Client{inner: c, spec: spec}, nil
}

func (c *Client) Close() error { return c.inner.Close() }

func (c *Client) Call(ctx context.Context, method string, req any) (any, error) {
	m, ok := c.spec.Method(method)
	if !ok {
		return nil, fmt.Errorf("generic: unknown method %s", method)
	}
	body, err := Encode(c.spec, m.Req, req)
	if err != nil {
		return nil, err
	}
	raw, err := c.inner.Call(ctx, method, body)
	if err != nil {
		return nil, err
	}
	return Decode(c.spec, m.Resp, raw)
}
