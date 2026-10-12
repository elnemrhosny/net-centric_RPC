package rpc

import (
	"fmt"
	"strings"
	"unicode"
)

func validateRequest(req Request) error {
	if strings.TrimSpace(req.Method) == "" {
		return fmt.Errorf("method name cannot be empty")
	}

	for _, r := range req.Method {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("method name contains whitespace or control characters")
		}
	}

	if len(req.Args) > maxArguments {
		return fmt.Errorf("too many arguments: maximum is %d", maxArguments)
	}

	totalSize := 0

	for i, arg := range req.Args {
		size := len(arg)
		if size > maxArgumentBytes {
			return fmt.Errorf("argument %d exceeds maximum size", i)
		}
		totalSize += size
	}

	// Include commas between arguments in the combined encoded size.
	if len(req.Args) > 1 {
		totalSize += len(req.Args) - 1
	}

	if totalSize > maxRequestBytes {
		return fmt.Errorf("combined arguments exceed maximum request size")
	}

	return nil
}
