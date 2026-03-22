package dbw

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	snowFlake       *Snowflake
	structMetaCache sync.Map // map[reflect.Type]*structMeta
	//config          *Config
	idGenerator = map[string]func() any{
		"snowflake":    func() any { return GetSnowflake().GetId() },
		"snowflakeStr": func() any { return fmt.Sprintf("%d", GetSnowflake().GetId()) },
	}
)

// RegisterIdGenerator 注册ID生成器
func RegisterIdGenerator(key string, fn func() any) {
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

func editStructProp[T any](obj *T, fieldName string, newValue any) error {
	if obj == nil {
		return errors.New("object is nil")
	}

	// 获取对象的反射值
	v := reflect.ValueOf(obj).Elem()

	// 检查是否是结构体
	if v.Kind() != reflect.Struct {
		return errors.New("obj must be a struct")
	}

	// 获取字段
	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return errors.New("field not found")
	}

	// 检查字段是否可设置
	if !field.CanSet() {
		return errors.New("field cannot be set")
	}

	// 获取字段类型
	fieldType := field.Type()

	// 检查字段类型是否在允许的范围内
	if !isAllowedType(fieldType) {
		return fmt.Errorf("field type %s is not allowed, only int, int64, *int, *int64, string, *string, time.Time, *time.Time are allowed", fieldType)
	}

	// 处理不同类型的赋值
	return assignValue(field, newValue)
}

// 检查类型是否在允许的范围内
func isAllowedType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int64, reflect.String:
		return true
	case reflect.Struct:
		// 检查是否是 time.Time 类型
		return t == reflect.TypeOf(time.Time{})
	case reflect.Ptr:
		elemType := t.Elem()
		return elemType.Kind() == reflect.Int ||
			elemType.Kind() == reflect.Int64 ||
			elemType.Kind() == reflect.String ||
			elemType == reflect.TypeOf(time.Time{})
	default:
		return false
	}
}

// 赋值函数，处理类型转换
func assignValue(field reflect.Value, newValue any) error {
	fieldType := field.Type()

	// 获取基础类型（如果是指针类型，则获取指向的类型）
	var baseType reflect.Type
	if fieldType.Kind() == reflect.Ptr {
		baseType = fieldType.Elem()
	} else {
		baseType = fieldType
	}

	// 处理nil值
	if newValue == nil {
		if fieldType.Kind() == reflect.Ptr {
			field.Set(reflect.Zero(fieldType))
			return nil
		}
		return errors.New("cannot set nil to non-pointer field")
	}

	// 根据字段的基础类型处理
	switch baseType.Kind() {
	case reflect.Int, reflect.Int64:
		// 尝试将新值转换为数字
		numValue, err := convertToNumber(newValue, baseType.Kind())
		if err != nil {
			return fmt.Errorf("cannot convert newValue to %s: %v", baseType.Kind(), err)
		}

		if fieldType.Kind() == reflect.Ptr {
			// 创建指针并赋值
			ptr := reflect.New(fieldType.Elem())
			ptr.Elem().Set(numValue)
			field.Set(ptr)
		} else {

			field.Set(numValue)
		}
		return nil

	case reflect.String:
		// 尝试将新值转换为字符串
		strValue, err := convertToString(newValue)
		if err != nil {
			return fmt.Errorf("cannot convert newValue to string: %v", err)
		}

		if fieldType.Kind() == reflect.Ptr {
			// 创建指针并赋值
			ptr := reflect.New(fieldType.Elem())
			ptr.Elem().Set(strValue)
			field.Set(ptr)
		} else {
			field.Set(strValue)
		}
		return nil

	case reflect.Struct:
		// 检查是否是 time.Time 类型
		if baseType == reflect.TypeOf(time.Time{}) {
			timeValue, err := convertToTime(newValue)
			if err != nil {
				return fmt.Errorf("cannot convert newValue to time.Time: %v", err)
			}

			if fieldType.Kind() == reflect.Ptr {
				// 创建指针并赋值
				ptr := reflect.New(fieldType.Elem())
				ptr.Elem().Set(timeValue)
				field.Set(ptr)
			} else {
				field.Set(timeValue)
			}
			return nil
		}
		return fmt.Errorf("unsupported struct type: %s", baseType)

	default:
		return fmt.Errorf("unsupported base type: %s", baseType.Kind())
	}
}

// 转换为数字类型
func convertToNumber(value any, targetType reflect.Kind) (reflect.Value, error) {
	switch v := value.(type) {
	case int:
		if targetType == reflect.Int64 {
			return reflect.ValueOf(int64(v)), nil
		}
		return reflect.ValueOf(v), nil

	case int64:
		if targetType == reflect.Int {
			return reflect.ValueOf(int(v)), nil
		}
		return reflect.ValueOf(v), nil

	case *int:
		if v == nil {
			return reflect.Value{}, errors.New("nil pointer")
		}
		if targetType == reflect.Int64 {
			return reflect.ValueOf(int64(*v)), nil
		}
		return reflect.ValueOf(*v), nil

	case *int64:
		if v == nil {
			return reflect.Value{}, errors.New("nil pointer")
		}
		if targetType == reflect.Int {
			return reflect.ValueOf(int(*v)), nil
		}
		return reflect.ValueOf(*v), nil

	case string:
		// 尝试将字符串转换为数字
		if targetType == reflect.Int {
			intVal, err := strconv.Atoi(v)
			if err != nil {
				// 尝试解析为int64，然后转换为int
				int64Val, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return reflect.Value{}, fmt.Errorf("cannot convert string '%s' to int: %v", v, err)
				}
				return reflect.ValueOf(int(int64Val)), nil
			}
			return reflect.ValueOf(intVal), nil
		} else if targetType == reflect.Int64 {
			int64Val, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("cannot convert string '%s' to int64: %v", v, err)
			}
			return reflect.ValueOf(int64Val), nil
		}

	case *string:
		if v == nil {
			return reflect.Value{}, errors.New("nil pointer")
		}
		// 递归调用，解引用字符串指针
		return convertToNumber(*v, targetType)

	default:
		// 尝试使用反射进行转换
		vv := reflect.ValueOf(value)
		if vv.Type().ConvertibleTo(reflect.TypeOf(int(0))) {
			intVal := vv.Convert(reflect.TypeOf(int(0))).Interface().(int)
			if targetType == reflect.Int64 {
				return reflect.ValueOf(int64(intVal)), nil
			}
			return reflect.ValueOf(intVal), nil
		}
	}

	return reflect.Value{}, fmt.Errorf("unsupported type for number conversion: %T", value)
}

// 转换为字符串类型
func convertToString(value any) (reflect.Value, error) {
	switch v := value.(type) {
	case string:
		return reflect.ValueOf(v), nil

	case *string:
		if v == nil {
			return reflect.Value{}, errors.New("nil pointer")
		}
		return reflect.ValueOf(*v), nil

	case int:
		return reflect.ValueOf(strconv.Itoa(v)), nil

	case int64:
		return reflect.ValueOf(strconv.FormatInt(v, 10)), nil

	case *int:
		if v == nil {
			return reflect.Value{}, errors.New("nil pointer")
		}
		return reflect.ValueOf(strconv.Itoa(*v)), nil

	case *int64:
		if v == nil {
			return reflect.Value{}, errors.New("nil pointer")
		}
		return reflect.ValueOf(strconv.FormatInt(*v, 10)), nil

	case time.Time:
		return reflect.ValueOf(v.Format(time.RFC3339)), nil

	case *time.Time:
		if v == nil {
			return reflect.Value{}, errors.New("nil pointer")
		}
		return reflect.ValueOf(v.Format(time.RFC3339)), nil

	default:
		// 尝试使用fmt.Sprintf转换
		return reflect.ValueOf(fmt.Sprintf("%v", value)), nil
	}
}

// 转换为 time.Time 类型
func convertToTime(value any) (reflect.Value, error) {
	switch v := value.(type) {
	case time.Time:
		return reflect.ValueOf(v), nil

	case *time.Time:
		if v == nil {
			return reflect.Value{}, errors.New("nil pointer")
		}
		return reflect.ValueOf(*v), nil

	case string:
		// 尝试多种时间格式进行解析
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02 15:04:05",
			"2006-01-02",
			time.RFC1123,
			time.RFC822,
		}

		var err error
		for _, format := range formats {
			t, err := time.Parse(format, v)
			if err == nil {
				return reflect.ValueOf(t), nil
			}
		}
		return reflect.Value{}, fmt.Errorf("cannot parse time string '%s', tried formats: %v", v, err)

	case *string:
		if v == nil {
			return reflect.Value{}, errors.New("nil pointer")
		}
		return convertToTime(*v)

	case int:
		// 假设为Unix时间戳（秒）
		return reflect.ValueOf(time.Unix(int64(v), 0)), nil

	case int64:
		// 假设为Unix时间戳（秒）
		return reflect.ValueOf(time.Unix(v, 0)), nil

	case float64:
		// 处理浮点数时间戳（可能包含毫秒/微秒）
		sec := int64(v)
		nsec := int64((v - float64(sec)) * 1e9)
		return reflect.ValueOf(time.Unix(sec, nsec)), nil

	default:
		// 尝试通过反射获取底层值
		rv := reflect.ValueOf(value)
		if rv.Type().ConvertibleTo(reflect.TypeOf(int64(0))) {
			int64Val := rv.Convert(reflect.TypeOf(int64(0))).Interface().(int64)
			return reflect.ValueOf(time.Unix(int64Val, 0)), nil
		}
		return reflect.Value{}, fmt.Errorf("unsupported type for time conversion: %T", value)
	}
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
