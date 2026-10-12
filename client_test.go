package rpc

import (
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// startServer launches a server on a free port and stops it when the test ends.
func startServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{}
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(s.Stop)
	return s
}

func newTestClient(s *Server) *Client {
	return NewClient(s.Address(), 2*time.Second)
}

func TestNewClientDefaultTimeout(t *testing.T) {
	if c := NewClient("x", 0); c.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %v, want 5s", c.Timeout)
	}
	if c := NewClient("x", -1); c.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %v, want 5s", c.Timeout)
	}
	if c := NewClient("x", time.Second); c.Timeout != time.Second {
		t.Fatalf("Timeout = %v, want 1s", c.Timeout)
	}
}

func TestCallSuccess(t *testing.T) {
	c := newTestClient(startServer(t))

	t.Run("no args", func(t *testing.T) {
		var got string
		if err := c.Call("NoArgs", &got); err != nil || got != "ok" {
			t.Fatalf("got %q, err %v", got, err)
		}
	})
	t.Run("add", func(t *testing.T) {
		var got int
		if err := c.Call("Add", &got, 2, 3); err != nil || got != 5 {
			t.Fatalf("got %d, err %v", got, err)
		}
	})
	t.Run("add negative", func(t *testing.T) {
		var got int
		if err := c.Call("Add", &got, -10, 4); err != nil || got != -6 {
			t.Fatalf("got %d, err %v", got, err)
		}
	})
	t.Run("echo special characters", func(t *testing.T) {
		in := `héllo "world" \ 你好 🚀`
		var got string
		if err := c.Call("Echo", &got, in); err != nil || got != in {
			t.Fatalf("got %q, err %v", got, err)
		}
	})
	t.Run("ping", func(t *testing.T) {
		var got string
		if err := c.Call("Ping", &got, 7); err != nil || got != "pong 7" {
			t.Fatalf("got %q, err %v", got, err)
		}
	})
	t.Run("error-only method succeeds", func(t *testing.T) {
		if err := c.Call("OnlyError", nil, false); err != nil {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("nil out ignores result", func(t *testing.T) {
		if err := c.Call("Add", nil, 1, 1); err != nil {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestCallServerErrors(t *testing.T) {
	c := newTestClient(startServer(t))

	tests := []struct {
		name    string
		method  string
		args    []any
		wantMsg string
	}{
		{"method returns error", "OnlyError", []any{true}, "requested failure"},
		{"ping error", "Ping", []any{-1}, "invalid id"},
		{"unknown method", "Missing", nil, "unknown method"},
		{"too few args", "Add", []any{1}, "expects 2 args"},
		{"too many args", "Add", []any{1, 2, 3}, "expects 2 args"},
		{"wrong arg type", "Add", []any{"x", 2}, "argument 0"},
		{"whitespace in method", "Add Method", nil, "method"},
		{"control char in method", "Add\n", nil, "method"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := c.Call(tc.method, nil, tc.args...)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("err = %q, want it to contain %q", err, tc.wantMsg)
			}
		})
	}
}

func TestCallClientSideErrors(t *testing.T) {
	c := newTestClient(startServer(t))

	if err := c.Call("", nil); err == nil {
		t.Fatal("empty method: want error")
	}
	if err := c.Call("Echo", nil, make(chan int)); err == nil {
		t.Fatal("unmarshalable argument: want error")
	}
	var wrong string
	if err := c.Call("Add", &wrong, 1, 2); err == nil {
		t.Fatal("decoding int into *string: want error")
	}
}

func TestCallValidationLimits(t *testing.T) {
	c := newTestClient(startServer(t))

	t.Run("too many arguments", func(t *testing.T) {
		args := make([]any, maxArguments+1)
		for i := range args {
			args[i] = i
		}
		err := c.Call("Add", nil, args...)
		if err == nil || !strings.Contains(err.Error(), "too many arguments") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("single argument too large", func(t *testing.T) {
		err := c.Call("Echo", nil, strings.Repeat("a", maxArgumentBytes+10))
		if err == nil || !strings.Contains(err.Error(), "exceeds maximum size") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("combined size too large", func(t *testing.T) {
		// 5 args of ~1000 bytes each: individually fine, together over 4096.
		args := make([]any, 5)
		for i := range args {
			args[i] = strings.Repeat("a", 1000)
		}
		err := c.Call("Add", nil, args...)
		if err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("request larger than read limit", func(t *testing.T) {
		if err := c.Call("Echo", nil, strings.Repeat("a", 20000)); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("server still healthy afterwards", func(t *testing.T) {
		var got int
		if err := c.Call("Add", &got, 1, 2); err != nil || got != 3 {
			t.Fatalf("got %d, err %v", got, err)
		}
	})
}

func TestRawProtocol(t *testing.T) {
	s := startServer(t)

	send := func(t *testing.T, payload string) Response {
		t.Helper()
		conn, err := net.DialTimeout("tcp", s.Address(), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		if _, err := conn.Write([]byte(payload)); err != nil {
			t.Fatal(err)
		}
		var resp Response
		if err := json.NewDecoder(conn).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return resp
	}

	t.Run("valid request", func(t *testing.T) {
		resp := send(t, `{"method":"Add","args":[2,3]}`+"\n")
		if !resp.OK || string(resp.Value) != "5" {
			t.Fatalf("resp = %+v", resp)
		}
	})
	t.Run("malformed json", func(t *testing.T) {
		resp := send(t, "{not json}\n")
		if resp.OK || !strings.Contains(resp.Error, "bad request") {
			t.Fatalf("resp = %+v", resp)
		}
	})
	t.Run("missing method", func(t *testing.T) {
		resp := send(t, `{"args":[]}`+"\n")
		if resp.OK || resp.Error == "" {
			t.Fatalf("resp = %+v", resp)
		}
	})
}

func TestCallConcurrent(t *testing.T) {
	c := newTestClient(startServer(t))

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var got int
			if err := c.Call("Add", &got, i, i); err != nil {
				errs <- err
				return
			}
			if got != 2*i {
				errs <- &mismatch{i, got}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

type mismatch struct{ i, got int }

func (m *mismatch) Error() string {
	return "Add(" + itoa(m.i) + "," + itoa(m.i) + ") returned " + itoa(m.got)
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestCallConnectionRefused(t *testing.T) {
	s := &Server{}
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	addr := s.Address()
	s.Stop()

	c := NewClient(addr, 500*time.Millisecond)
	err := c.Call("NoArgs", nil)
	if err == nil || !strings.Contains(err.Error(), "connect to") {
		t.Fatalf("err = %v, want connect error", err)
	}
}

func TestCallTimeout(t *testing.T) {
	// A listener that accepts but never replies.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var (
		mu    sync.Mutex
		conns []net.Conn
	)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			c.Close()
		}
	})

	c := NewClient(ln.Addr().String(), 200*time.Millisecond)
	start := time.Now()
	err = c.Call("NoArgs", nil)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("err = %v, want timeout", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("took %v, deadline not enforced", d)
	}
}

func TestCallAfterStopAndRestart(t *testing.T) {
	s := &Server{}
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	c := NewClient(s.Address(), time.Second)

	var got int
	if err := c.Call("Add", &got, 1, 1); err != nil || got != 2 {
		t.Fatalf("before stop: got %d, err %v", got, err)
	}

	s.Stop()
	if err := c.Call("Add", &got, 1, 1); err == nil {
		t.Fatal("after stop: want error")
	}

	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("restart: %v", err)
	}
	defer s.Stop()
	c = NewClient(s.Address(), time.Second)
	if err := c.Call("Add", &got, 4, 5); err != nil || got != 9 {
		t.Fatalf("after restart: got %d, err %v", got, err)
	}
}

func TestStopWaitsForNoLeaks(t *testing.T) {
	s := &Server{}
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	c := NewClient(s.Address(), time.Second)
	for i := 0; i < 20; i++ {
		if err := c.Call("NoArgs", nil); err != nil {
			t.Fatal(err)
		}
	}

	done := make(chan struct{})
	go func() { s.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop hung: a goroutine is not calling wg.Done")
	}
}
