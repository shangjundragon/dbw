package dbw

import (
	"fmt"
	"strings"
	"unicode"
)

// Tabler is an interface for custom table name mapping.
type Tabler interface {
	TableName() string
}

// getTableName converts a Go type name to snake_case table name.
// Supports abbreviation handling: HTTPServer → http_server, OAuthClient → oauth_client
func getTableName[T any]() string {
	var t T
	name := fmt.Sprintf("%T", t)

	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}
	if name == "" {
		return "unknown"
	}

	runes := []rune(name)
	var result []rune

	for i, r := range runes {
		if unicode.IsUpper(r) {
			shouldInsertUnderscore := false
			if i > 0 {
				prev := runes[i-1]
				nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if unicode.IsLower(prev) {
					shouldInsertUnderscore = true
				}
				if unicode.IsUpper(prev) && nextIsLower {
					shouldInsertUnderscore = true
				}
			}
			if shouldInsertUnderscore {
				result = append(result, '_')
			}
		}
		result = append(result, unicode.ToLower(r))
	}

	return string(result)
}

// camelToSnake converts a CamelCase string to snake_case.
func camelToSnake(s string) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	var result []rune

	for i, r := range runes {
		if unicode.IsUpper(r) {
			shouldInsertUnderscore := false
			if i > 0 {
				prev := runes[i-1]
				nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if unicode.IsLower(prev) {
					shouldInsertUnderscore = true
				}
				if unicode.IsUpper(prev) && nextIsLower {
					shouldInsertUnderscore = true
				}
			}
			if shouldInsertUnderscore {
				result = append(result, '_')
			}
		}
		result = append(result, unicode.ToLower(r))
	}

	return string(result)
}
