package dbw

import (
	"context"
	"encoding/json"
	"fmt"
)

var logFn func(sqlStr string, args []any, ctx context.Context)

// SetLogFn sets a custom logging function for SQL debug output.
func SetLogFn(fn func(sqlStr string, args []any, ctx context.Context)) {
	logFn = fn
}

func debugLog(config *Config, ctx context.Context, sqlStr string, args []any) {
	if config == nil || !config.Debug {
		return
	}
	if logFn != nil {
		logFn(sqlStr, args, ctx)
		return
	}
	if sqlStr == "" {
		fmt.Println("sqlStr is empty")
		return
	}
	marshal, _ := json.Marshal(args)
	fmt.Printf("SQL: %s\nArgs: %v\n", sqlStr, string(marshal))
}
