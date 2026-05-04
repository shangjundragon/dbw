package dbw

import (
	"reflect"
	"strconv"
	"time"
)

// getTime returns the current time based on the tag value ("milli" for UnixMilli, otherwise time.Time).
func getTime(timeTagValue string) any {
	switch timeTagValue {
	case "milli":
		return time.Now().UnixMilli()
	default:
		return time.Now()
	}
}

// convertDefaultValue converts a string default value to the appropriate Go type.
func convertDefaultValue(defaultValue string, targetType reflect.Type) any {
	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}
	switch targetType.Kind() {
	case reflect.Int:
		if num, err := strconv.Atoi(defaultValue); err == nil {
			return num
		}
	case reflect.Int8:
		if num, err := strconv.ParseInt(defaultValue, 10, 8); err == nil {
			return int8(num)
		}
	case reflect.Int16:
		if num, err := strconv.ParseInt(defaultValue, 10, 16); err == nil {
			return int16(num)
		}
	case reflect.Int32:
		if num, err := strconv.ParseInt(defaultValue, 10, 32); err == nil {
			return int32(num)
		}
	case reflect.Int64:
		if num, err := strconv.ParseInt(defaultValue, 10, 64); err == nil {
			return num
		}
	case reflect.Uint:
		if num, err := strconv.ParseUint(defaultValue, 10, strconv.IntSize); err == nil {
			return uint(num)
		}
	case reflect.Uint8:
		if num, err := strconv.ParseUint(defaultValue, 10, 8); err == nil {
			return uint8(num)
		}
	case reflect.Uint16:
		if num, err := strconv.ParseUint(defaultValue, 10, 16); err == nil {
			return uint16(num)
		}
	case reflect.Uint32:
		if num, err := strconv.ParseUint(defaultValue, 10, 32); err == nil {
			return uint32(num)
		}
	case reflect.Uint64:
		if num, err := strconv.ParseUint(defaultValue, 10, 64); err == nil {
			return num
		}
	case reflect.Float32:
		if num, err := strconv.ParseFloat(defaultValue, 32); err == nil {
			return float32(num)
		}
	case reflect.Float64:
		if num, err := strconv.ParseFloat(defaultValue, 64); err == nil {
			return num
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(defaultValue); err == nil {
			return b
		}
	case reflect.String:
		return defaultValue
	default:
		return defaultValue
	}
	return defaultValue
}
