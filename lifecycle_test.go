package rpc

import (
	"net"
	"strings"
	"testing"
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

	//8 - Restart works
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
