package rpc

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServerLifecycle(t *testing.T) {
	s := &Server{}

	//1 - Address before Start
	if got := s.Address(); got != "" {
		t.Fatalf("Address before Start = %q , want empty", got)
	}

	//2 - start on port 0
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Start failed : %v", err)
	}

	//3 - Address reflects OS chosen port
	addr := s.Address()
	if addr == "" {
		t.Fatal("Address after Start is empty")
	}
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Fatalf("Address : %q , want 127.0.0.1:<port>", addr)
	}
	if strings.HasSuffix(addr, ":0") {
		t.Fatalf("Address = %q , port 0 was not replaced", addr)
	}

	//4 - Duplicate Start is rejected
	if err := s.Start("127.0.0.1:0"); err == nil {
		t.Fatal("second Start succeeded , want error")
	}

	// 5 - Stop
	s.Stop()

	//6 - Address is empty after Stop
	if got := s.Address(); got != "" {
		t.Fatalf("Address after Stop = %q , want empty", got)
	}

	//8 - Stop is idempotent
	s.Stop()

	//9 - Restart works
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("restart failed : %v", err)
	}
	s.Stop()
}

func TestStopBeforeStart(t *testing.T) {
	s := &Server{}
	s.Stop() //must not panic or hang
	if got := s.Address(); got != "" {
		t.Fatalf("Address = %q , want empty", got)
	}
}

func TestServerAcceptsTCP(t *testing.T) {
	s := &Server{}
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	conn, err := net.Dial("tcp", s.Address())
	if err != nil {
		t.Fatalf("dial failed : %v", err)
	}
	conn.Close()
}
func TestMalformedJSON(t *testing.T) {
	srv := &Server{service: &Service{}}
	if err := srv.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Not valid JSON
	conn.Write([]byte("{not json\n"))

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.OK {
		t.Fatal("malformed JSON returned ok=true, want error")
	}
}
func TestOversizedArgs(t *testing.T) {
	srv := &Server{service: &Service{}}
	if err := srv.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	c := NewClient(srv.Address(), time.Second)
	huge := strings.Repeat("a", 100_000) // 100 KB, way over any limit
	var out string
	if err := c.Call("Echo", &out, huge); err == nil {
		t.Fatal("oversized arg returned nil, want error")
	}
}

func TestInvokePanic(t *testing.T) {
	s := &Server{service: &Service{}}

	_, err := s.invoke(Request{Method: "Panic"})
	if err == nil {
		t.Fatal("panic method returned nil error")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Fatalf("error = %q, want it to mention panic", err)
	}
}

func TestConcurrentCalls(t *testing.T) {
	srv := &Server{service: &Service{}}
	if err := srv.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	c := NewClient(srv.Address(), 2*time.Second)
	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var got int
			if err := c.Call("Add", &got, i, 1); err != nil {
				errs <- err
				return
			}
			if got != i+1 {
				errs <- fmt.Errorf("Add(%d,1)=%d, want %d", i, got, i+1)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestStopWaitsForHandlers(t *testing.T) {
	srv := &Server{service: &Service{}}
	if err := srv.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		c := NewClient(srv.Address(), time.Second)
		var got string
		_ = c.Call("Slow", &got)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond) // let the handler start
	start := time.Now()
	srv.Stop()
	elapsed := time.Since(start)

	if elapsed < 200*time.Millisecond {
		t.Fatalf("Stop returned in %v, expected to wait for handler", elapsed)
	}
	<-done
}

func TestClientTimeout(t *testing.T) {
	srv := &Server{service: &Service{}}
	if err := srv.Start("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	c := NewClient(srv.Address(), 100*time.Millisecond)
	err := c.Call("Slow", nil)
	if err == nil {
		t.Fatal("expected timeout, got nil")
	}
}
func TestValidateRequestLimits(t *testing.T) {
	big := json.RawMessage(strings.Repeat("a", maxArgumentBytes+1))
	err := validateRequest(Request{Method: "Echo", Args: []json.RawMessage{big}})
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Fatalf("got %v, want 'exceeds maximum size'", err)
	}
}
