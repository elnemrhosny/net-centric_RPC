package rpc

import (
	"encoding/json"
	"fmt"
	"reflect"
)

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// invoke finds req.Method on the service via reflection, decodes the args,
// calls it, and returns its result (nil when the method has none).
func (s *Server) invoke(req Request) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			result, err = nil, fmt.Errorf("method panicked: %v", r)
		}
	}()

	m := reflect.ValueOf(&Service{}).MethodByName(req.Method)
	if !m.IsValid() {
		return nil, fmt.Errorf("unknown method %q", req.Method)
	}
	mt := m.Type()
	if len(req.Args) != mt.NumIn() {
		return nil, fmt.Errorf("method %q expects %d args, got %d", req.Method, mt.NumIn(), len(req.Args))
	}

	in := make([]reflect.Value, mt.NumIn())
	for i := range in {
		p := reflect.New(mt.In(i))
		if err := json.Unmarshal(req.Args[i], p.Interface()); err != nil {
			return nil, fmt.Errorf("argument %d: cannot decode as %s: %v", i, mt.In(i), err)
		}
		in[i] = p.Elem()
	}

	out := m.Call(in)
	if n := len(out); n > 0 && mt.Out(n-1) == errorType {
		if e, _ := out[n-1].Interface().(error); e != nil {
			return nil, e
		}
		out = out[:n-1]
	}
	switch len(out) {
	case 0:
		return nil, nil
	case 1:
		return out[0].Interface(), nil
	default:
		return nil, fmt.Errorf("method %q has unsupported return shape", req.Method)
	}
}
