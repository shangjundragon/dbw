package dbw

import (
	"context"
	"fmt"
	"reflect"
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

// SetFieldValue sets a struct field via reflection with automatic type conversion.
// It handles int/uint/float/string conversions to avoid "value of type X is not assignable to type Y" errors.
// Useful in EntityHook and general reflection-based field assignment.
func SetFieldValue(field reflect.Value, val any) {
	if val == nil || !field.IsValid() {
		return
	}
	v := reflect.ValueOf(val)
	if v.Type() == field.Type() {
		field.Set(v)
		return
	}
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(v.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetInt(int64(v.Uint()))
		case reflect.Float32, reflect.Float64:
			field.SetInt(int64(v.Float()))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetUint(uint64(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetUint(v.Uint())
		case reflect.Float32, reflect.Float64:
			field.SetUint(uint64(v.Float()))
		}
	case reflect.Float32, reflect.Float64:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetFloat(float64(v.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetFloat(float64(v.Uint()))
		case reflect.Float32, reflect.Float64:
			field.SetFloat(v.Float())
		}
	case reflect.String:
		field.SetString(fmt.Sprint(val))
	}
}
