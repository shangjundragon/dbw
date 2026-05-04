package dbw

import (
	"context"
	"fmt"
	"time"
)

// GetInt64Ptr returns a pointer to the given int64 value.
func GetInt64Ptr(i int64) *int64 { return &i }

// GetIntPtr returns a pointer to the given int value.
func GetIntPtr(i int) *int { return &i }

// GetStringPtr returns a pointer to the given string value.
func GetStringPtr(s string) *string { return &s }

// GetFloat64Ptr returns a pointer to the given float64 value.
func GetFloat64Ptr(f float64) *float64 { return &f }

func sliceSplit[T any](sli []T, size int) ([][]T, error) {
	if size <= 0 {
		return nil, fmt.Errorf("size must be positive, got %d", size)
	}
	if len(sli) == 0 {
		return [][]T{}, nil
	}
	result := make([][]T, 0, (len(sli)+size-1)/size)
	for i := 0; i < len(sli); i += size {
		end := i + size
		if end > len(sli) {
			end = len(sli)
		}
		result = append(result, sli[i:end])
	}
	return result, nil
}

// GetContextWithTimeout creates a context with a timeout from the background context.
func GetContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
