package rpc

import (
	"errors"
	"net"
	"sync"
)

type Server struct {
	mu       sync.Mutex //locking goroutines to prevent race conditions
	listener net.Listener
	// service  *Service
	running bool           //lets start reject a second call while server is already running
	wg      sync.WaitGroup //tracks the accept goroutine and all active connection handlers , Stop will call wg.wait to wait for them to finish
}

func (s *Server) Start(addr string) error {
	//lock mutex
	s.mu.Lock()
	defer s.mu.Unlock()

	//return error if server is already running
	if s.running {
		return errors.New("server already running")
	}

	//listen on the provided address
	ln, err := net.Listen("tcp", addr) //listen on the provided address

	if err != nil {
		return err
	}

	//set listener and running and add 1 to wait group for the accept loop
	//wg.Add is necessary because Stop function has to wait for other go routines like acceptLoop
	//before closing to server
	s.listener = ln
	s.running = true
	s.wg.Add(1)

	go s.acceptLoop()
	return nil
}

func (s *Server) Address() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running { // if not listening or listener is nil return ""
		return ""
	}
	return s.listener.Addr().String()
}

// function for accepting request
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		//accepts connection on recieving requests
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener was closed by stop or a real error
		}

		s.mu.Lock()
		if !s.running {
			s.mu.Unlock()
			conn.Close()
			return
		}
		// s.wg.Add(1) //dont forget to defer this inside s.handle
		s.mu.Unlock()
		// go s.handle()
	}
}

func (s *Server) Stop() {
	s.mu.Lock()

	// if server is already not running
	if !s.running {
		s.mu.Unlock()
		return
	}

	//set running to false and close the listener
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}

	s.mu.Unlock()
	//wait for all working goroutines before returning and closing the server
	s.wg.Wait()

}
