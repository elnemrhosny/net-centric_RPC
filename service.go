package rpc

import (
	"fmt"
	"time"
)

type Service struct{}

func (s *Service) NoArgs() string           { return "ok" }
func (s *Service) Add(left, right int) int  { return left + right }
func (s *Service) Echo(value string) string { return value }
func (s *Service) OnlyError(shouldFail bool) error {
	if shouldFail {
		return fmt.Errorf("requested failure")
	}
	return nil
}
func (s *Service) Ping(id int) (string, error) {
	if id < 0 {
		return "", fmt.Errorf("invalid id %d", id)
	}
	return fmt.Sprintf("pong %d", id), nil
}

// for tests
func (s *Service) Slow() string {
	time.Sleep(300 * time.Millisecond)
	return "ok"
}

func (s *Service) Panic() string {
	panic("boom")
}
