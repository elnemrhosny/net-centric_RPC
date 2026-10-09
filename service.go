package rpc

import "fmt"

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
