package dbw

import (
	"fmt"
	"sync"
)

var (
	idGeneratorMu sync.RWMutex
	idGenerator   = map[string]func() any{
		"snowflake":    func() any { return GetSnowflake().GetId() },
		"snowflakeStr": func() any { return fmt.Sprintf("%d", GetSnowflake().GetId()) },
	}
)

// RegisterIdGenerator registers a custom ID generator function by name.
func RegisterIdGenerator(key string, fn func() any) {
	idGeneratorMu.Lock()
	defer idGeneratorMu.Unlock()
	idGenerator[key] = fn
}
