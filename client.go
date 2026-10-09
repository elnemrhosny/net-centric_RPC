package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

type Client struct {
	Addr    string
	Timeout time.Duration
}

func NewClient(addr string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{Addr: addr, Timeout: timeout}
}

// Call invokes method remotely and decodes the result into out (nil allowed).
func (c *Client) Call(method string, out any, args ...any) error {
	if method == "" {
		return errors.New("empty method name")
	}
	req := Request{Method: method, Args: make([]json.RawMessage, len(args))}
	for i, a := range args {
		b, err := json.Marshal(a)
		if err != nil {
			return fmt.Errorf("encode argument %d: %w", i, err)
		}
		req.Args[i] = b
	}

	conn, err := net.DialTimeout("tcp", c.Addr, c.Timeout)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", c.Addr, err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(c.Timeout)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	if err := json.NewEncoder(conn).Encode(&req); err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return fmt.Errorf("timeout waiting for response: %w", err)
		}
		return fmt.Errorf("decode response: %w", err)
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	if out == nil || len(resp.Value) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Value, out); err != nil {
		return fmt.Errorf("decode result into %T: %w", out, err)
	}
	return nil
}
