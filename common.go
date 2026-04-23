package dbw

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type PlaceholderConverter func(string) string

// MySQLConverter 保持 '?' 不变
func MySQLConverter(sql string) string { return sql }

func PostgreSConverter(sql string) string {
	var buf strings.Builder
	paramIndex := 1
	for i := 0; i < len(sql); i++ {
		if sql[i] == '?' {
			fmt.Fprintf(&buf, "$%d", paramIndex)
			paramIndex++
		} else {
			buf.WriteByte(sql[i])
		}
	}
	return buf.String()
}

type Config struct {
	LogicDeleteValue    string // 逻辑删除值 1表示删除了
	LogicNotDeleteValue string // 逻辑删除值 0表示未删除
	Debug               bool   // 是否调试模式，默认为false
	Db                  *sql.DB
	DriverName          string // 数据库驱动名 影响分页语句 可选：mysql、postgres、sqlite	oracle、sqlserver
	// 分页拦截器
	PageInterceptor      func(sqlStr string, pageNum int, pageSize int) (finalSql string)
	PlaceholderConverter PlaceholderConverter
}

func NewConfig(fn func(config *Config)) *Config {
	var c = Config{
		LogicDeleteValue:     "1",
		LogicNotDeleteValue:  "0",
		PlaceholderConverter: MySQLConverter,
	}
	fn(&c)

	if c.Db == nil {
		panic("database connection is required")
	}
	if c.LogicDeleteValue == "" {
		panic("logic delete value is required")
	}
	if c.LogicNotDeleteValue == "" {
		panic("logic not delete value is required")
	}
	return &c
}

var (
	snowFlakeMu     sync.RWMutex // 保护 snowFlake 的读写锁
	snowFlake       *Snowflake
	structMetaCache sync.Map     // map[reflect.Type]*structMeta
	idGeneratorMu   sync.RWMutex // 保护 idGenerator 的读写锁
	idGenerator     = map[string]func() any{
		"snowflake":    func() any { return GetSnowflake().GetId() },
		"snowflakeStr": func() any { return fmt.Sprintf("%d", GetSnowflake().GetId()) },
	}
)

// RegisterIdGenerator 注册 ID 生成器
func RegisterIdGenerator(key string, fn func() any) {
	idGeneratorMu.Lock()
	defer idGeneratorMu.Unlock()
	idGenerator[key] = fn
}

// getTime 根据时间标签获取时间
func getTime(timeTagValue string) any {
	switch timeTagValue {
	case "milli":
		return time.Now().UnixMilli()
	default:
		return time.Now()
	}
}

// getTableName 获取表名（增强版）
// 将 Go 类型名（PascalCase）转换为数据库表名（snake_case）
// 示例：
//
//	UserInfo       → user_info
//	HTTPServer     → http_server
//	XMLParser      → xml_parser
//	OAuthClient    → oauth_client
func getTableName[T any]() string {
	var t T
	name := fmt.Sprintf("%T", t)

	// 去除包路径
	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}

	if name == "" {
		return "unknown"
	}

	runes := []rune(name)
	var result []rune

	for i, r := range runes {
		// 如果是大写字母
		if unicode.IsUpper(r) {
			// 规则：在以下情况前加下划线：
			// 1. 不是第一个字符
			// 2. 前一个字符不是大写（如 User → u_ser? 不加；但 Info → _info）
			// 3. 或者后面还有小写字母（说明当前大写是缩写结尾，如 HTTPRequest 中的 P 后是 R（大写），但 Request 的 R 后是 e（小写）→ 应在 R 前加 _）

			shouldInsertUnderscore := false

			if i > 0 {
				prev := runes[i-1]
				nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])

				// 情况1：前一个是小写（User + Info → user_info）
				if unicode.IsLower(prev) {
					shouldInsertUnderscore = true
				}
				// 情况2：前一个是大写，但当前字母后跟小写字母（HTTPRequest → http_request）
				//        即：当前是缩写的最后一个大写字母
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

	tableName := string(result)

	// 可选：自动转为复数（简单规则，仅加 's'，适用于大多数情况）
	// 如需更智能的复数（如 "user" → "users", "category" → "categories"），建议使用专用库（如 github.com/gertd/go-pluralize）
	// 这里暂不启用，保持中立。如需启用，取消注释下一行：
	// tableName = pluralize(tableName)

	return tableName
}

// camelToSnake 将驼峰命名转换为下划线命名
func camelToSnake(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

var logFn func(sqlStr string, args []any, ctx context.Context)

// SetLogFn 设置日志函数
func SetLogFn(fn func(sqlStr string, args []any, ctx context.Context)) {
	logFn = fn
}
func (q *DbWrapper[T]) PrintDebugSql(sqlStr string, args []any) *DbWrapper[T] {
	if logFn != nil {
		logFn(sqlStr, args, q.ctx)
		return q
	}
	if sqlStr == "" {
		fmt.Println("sqlStr is empty")
		return q
	}
	if len(args) == 0 {
		fmt.Println("args is empty")
	}
	marshal, _ := json.Marshal(args)
	fmt.Printf("SQL: %s \nArgs: %v\n", sqlStr, string(marshal))
	return q
}

func GetInt64Ptr(i int64) *int64 {
	return &i
}
func GetIntPtr(i int) *int {
	return &i
}

func GetStringPtr(s string) *string {
	return &s
}
func GetFloat64Ptr(f float64) *float64 {
	return &f
}

// GetContextWithTimeout 创建带超时的上下文（用于测试）
func GetContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func sliceSplit[T any](sli []T, size int) ([][]T, error) {
	// 参数校验
	if size <= 0 {
		return nil, fmt.Errorf("size must be positive, got %d", size)
	}

	if len(sli) == 0 {
		return [][]T{}, nil
	}

	// 预分配结果切片容量，减少扩容
	result := make([][]T, 0, (len(sli)+size-1)/size)

	for i := 0; i < len(sli); i += size {
		// 计算当前子切片的结束位置
		end := i + size
		if end > len(sli) {
			end = len(sli)
		}

		// 直接切片，避免额外的内存分配和拷贝
		result = append(result, sli[i:end])
	}

	return result, nil
}

// convertDefaultValue 转换默认值为合适的类型
func convertDefaultValue(defaultValue string, targetType reflect.Type) any {
	// 如果是指针类型，先获取元素类型
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
