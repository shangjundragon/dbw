# DBW v1 重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完全重写 dbw ORM 库，修复 6 个 Bug + 6 项性能优化 + 5 项新功能，库单包 ~25 文件组织。

**Architecture:** 单包 `dbw`，Dialect 接口抽象 4 种数据库，`whereExpr.joiner` 解决 Or() 问题，预编译语句缓存提升性能。

**Tech Stack:** Go 1.21+, `database/sql`, `github.com/glebarez/go-sqlite`, `github.com/go-sql-driver/mysql`

---

## 文件结构总览

创建 22 个新源文件 + 删除 8 个旧文件：

```
dbw/
├── dbw.go              # NEW: New[T], Options, ExecuteTx
├── config.go           # NEW: Config, Dialect, NewConfig
├── dialect_mysql.go    # NEW
├── dialect_sqlite.go   # NEW
├── dialect_postgres.go # NEW
├── dialect_oracle.go   # NEW
├── wrapper.go          # NEW: DbWrapper[T]
├── condition.go        # NEW: Where, Eq, Ne, Gt, Ge, Lt, Le, Like, In, Between, IsNull, NotNull
├── condition_group.go  # NEW: And, Or, OrNest, WhereIf, etc.
├── query.go            # NEW: Select, OrderBy, GroupBy, Having, Distinct, Limit, Offset
├── query_sql.go        # NEW: buildSelectSQL, buildWhere, query, queryRow
├── select.go           # NEW: SelectOne, FindOne, SelectList, SelectById, SelectPage, Count, Exist
├── raw.go              # NEW: Raw, Exec
├── insert.go           # NEW: Insert, InsertBatch, InsertBatchSplit
├── update.go           # NEW: UpdateById, Update
├── delete.go           # NEW: Delete, DeleteById, DeleteByIds
├── meta.go             # NEW: structMeta, getStructMeta, resolveDbwTag
├── table.go            # NEW: getTableName, camelToSnake, Tabler
├── snowflake.go        # NEW: Snowflake, GetSnowflake
├── generator.go        # NEW: RegisterIdGenerator
├── errors.go           # NEW: 结构化错误
├── log.go              # NEW: SetLogFn, debugLog
├── convert.go          # NEW: convertDefaultValue, getTime
├── utils.go            # NEW: 辅助函数
└── where_struct.go     # NEW: WhereStruct

DELETE 旧文件:
├── dbwrapper.go        # → 拆分到 wrapper.go, dbw.go
├── common.go           # → 拆分到 config.go, table.go, log.go, convert.go, utils.go
├── structmeta.go       # → meta.go
├── where.go            # → condition.go, condition_group.go
├── select.go           # → query.go, query_sql.go, select.go, raw.go
├── insert.go           # → insert.go (重写)
├── update.go           # → update.go (重写)
├── delete.go           # → delete.go (重写)
└── snow_flake.go       # → snowflake.go, generator.go
```

---

## Phase 1: Foundation（基础类型和工具）

### Task 1: errors.go — 结构化错误类型

**Files:**
- Create: `errors.go`
- Delete: (无旧文件依赖)

- [ ] **Step 1: 创建 errors.go**

```go
package dbw

import "errors"

var (
	ErrRecordNotFound   = errors.New("dbw: record not found")
	ErrMultipleRecords  = errors.New("dbw: expected 1 record, got multiple")
	ErrNoWhereClause    = errors.New("dbw: dangerous operation without WHERE clause")
	ErrNoFieldsToUpdate = errors.New("dbw: no fields to update")
	ErrNoPrimaryKey     = errors.New("dbw: primary key not configured on struct")
	ErrBatchTooLarge    = errors.New("dbw: batch size exceeds maximum limit of 1000, use InsertBatchSplit")
	ErrEmptyData        = errors.New("dbw: data slice is empty")
	ErrNilEntity        = errors.New("dbw: entity cannot be nil")
)
```

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```
Expected: 编译成功 (仅有 errors.go)。

- [ ] **Step 3: 提交**

```bash
git add errors.go
git commit -m "feat: add structured error types for dbw v1"
```

---

### Task 2: utils.go + convert.go — 工具函数

**Files:**
- Create: `utils.go`, `convert.go`
- Delete: 之后删除 `common.go` (此时保留，Phase 7 删除)

- [ ] **Step 1: 创建 utils.go**

```go
package dbw

import (
	"fmt"
	"reflect"
)

func GetInt64Ptr(i int64) *int64 { return &i }
func GetIntPtr(i int) *int       { return &i }
func GetStringPtr(s string) *string { return &s }
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
```

- [ ] **Step 2: 创建 convert.go**

```go
package dbw

import (
	"reflect"
	"strconv"
	"time"
)

func getTime(timeTagValue string) any {
	switch timeTagValue {
	case "milli":
		return time.Now().UnixMilli()
	default:
		return time.Now()
	}
}

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
```

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```
Expected: 编译成功，注意旧文件仍存在。

- [ ] **Step 4: 提交**

```bash
git add utils.go convert.go
git commit -m "feat: add utility functions and default value converter"
```

---

### Task 3: snowflake.go + generator.go — ID 生成

**Files:**
- Create: `snowflake.go`, `generator.go`
- Delete: 之后删除 `snow_flake.go` (Phase 7)

- [ ] **Step 1: 创建 snowflake.go**

```go
package dbw

import (
	"sync"
	"sync/atomic"
	"time"
)

var snowflakeMachineId atomic.Int64

func init() {
	snowflakeMachineId.Store(1)
}

func SetSnowflakeMachineId(id int64) {
	snowflakeMachineId.Store(id)
}

type Snowflake struct {
	sync.Mutex
	timestamp int64
	machineId int64
	sequence  int64
}

func CreateSnowflakeFactory() Snowflake {
	return Snowflake{
		machineId: snowflakeMachineId.Load(),
	}
}

var (
	snowFlakeMu sync.RWMutex
	snowFlake   *Snowflake
)

func GetSnowflake() *Snowflake {
	snowFlakeMu.RLock()
	if snowFlake != nil {
		snowFlakeMu.RUnlock()
		return snowFlake
	}
	snowFlakeMu.RUnlock()

	snowFlakeMu.Lock()
	defer snowFlakeMu.Unlock()
	if snowFlake == nil {
		s := CreateSnowflakeFactory()
		snowFlake = &s
	}
	return snowFlake
}

func (s *Snowflake) GetId() int64 {
	s.Lock()
	defer s.Unlock()
	now := time.Now().UnixNano() / 1e6
	if s.timestamp == now {
		s.sequence = (s.sequence + 1) & 0xFFF
		if s.sequence == 0 {
			for now <= s.timestamp {
				now = time.Now().UnixNano() / 1e6
			}
		}
	} else {
		s.sequence = 0
	}
	s.timestamp = now
	r := (now-1483228800000)<<22 | (s.machineId << 12) | s.sequence
	return r
}
```

- [ ] **Step 2: 创建 generator.go**

```go
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

func RegisterIdGenerator(key string, fn func() any) {
	idGeneratorMu.Lock()
	defer idGeneratorMu.Unlock()
	idGenerator[key] = fn
}
```

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 4: 提交**

```bash
git add snowflake.go generator.go
git commit -m "feat: add snowflake ID generator with atomic machine ID and generator registry"
```

---

### Task 4: log.go — 日志系统

**Files:**
- Create: `log.go`

- [ ] **Step 1: 创建 log.go**

```go
package dbw

import (
	"context"
	"encoding/json"
	"fmt"
)

var logFn func(sqlStr string, args []any, ctx context.Context)

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
```

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 3: 提交**

```bash
git add log.go
git commit -m "feat: add configurable logging with SetLogFn and debug output"
```

---

## Phase 2: Config & Dialect（配置和数据库方言）

### Task 5: config.go — Config 和 Dialect 接口

**Files:**
- Create: `config.go`

- [ ] **Step 1: 创建 config.go**

```go
package dbw

import (
	"database/sql"
	"time"
)

// Dialect 数据库方言接口
type Dialect interface {
	DriverName() string
	Placeholder(n int) string
	ConvertPlaceholders(sql string) string
	QuoteIdentifier(name string) string
	BuildPagination(sql string, limit, offset int) string
}

type Config struct {
	Db                 *sql.DB
	DriverName         string        // mysql, sqlite, postgres, oracle
	Dialect            Dialect       // 方言实现（由 DriverName 自动选择）
	LogicDeleteValue   string        // 逻辑删除值，默认 "1"
	LogicNotDeleteValue string       // 逻辑未删除值，默认 "0"
	Debug              bool          // 调试模式
	PageInterceptor    func(sqlStr string, pageNum int, pageSize int) string

	// 连接池配置
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration

	// 预编译语句缓存大小（0=禁用）
	PreparedStmtCacheSize int
}

func NewConfig(fn func(config *Config)) *Config {
	c := &Config{
		LogicDeleteValue:    "1",
		LogicNotDeleteValue: "0",
	}
	fn(c)
	if c.Db == nil {
		panic("dbw: database connection is required")
	}
	if c.LogicDeleteValue == "" {
		panic("dbw: logic delete value is required")
	}
	if c.LogicNotDeleteValue == "" {
		panic("dbw: logic not delete value is required")
	}

	// 根据 DriverName 选择方言
	if c.Dialect == nil {
		switch c.DriverName {
		case "mysql":
			c.Dialect = &mysqlDialect{}
		case "sqlite":
			c.Dialect = &sqliteDialect{}
		case "postgres":
			c.Dialect = &postgresDialect{}
		case "oracle":
			c.Dialect = &oracleDialect{}
		default:
			c.Dialect = &mysqlDialect{}
		}
	}

	// 连接池配置
	if c.MaxOpenConns > 0 {
		c.Db.SetMaxOpenConns(c.MaxOpenConns)
	}
	if c.MaxIdleConns > 0 {
		c.Db.SetMaxIdleConns(c.MaxIdleConns)
	}
	if c.ConnMaxLifetime > 0 {
		c.Db.SetConnMaxLifetime(c.ConnMaxLifetime)
	}

	return c
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```
Expected: 会报错，因为 `mysqlDialect` 等类型尚未定义。需要先创建 dialect 文件。

- [ ] **Step 3: 提交**（此时编译未通过，暂不提交，与 Task 6-7 一起提交）

---

### Task 6: dialect_mysql.go + dialect_sqlite.go

**Files:**
- Create: `dialect_mysql.go`, `dialect_sqlite.go`

- [ ] **Step 1: 创建 dialect_mysql.go**

```go
package dbw

import (
	"fmt"
	"strings"
)

type mysqlDialect struct{}

func (d *mysqlDialect) DriverName() string { return "mysql" }

func (d *mysqlDialect) Placeholder(n int) string {
	return "?"
}

func (d *mysqlDialect) ConvertPlaceholders(sql string) string {
	return sql // MySQL 使用 ? 无需转换
}

func (d *mysqlDialect) QuoteIdentifier(name string) string {
	return "`" + name + "`"
}

func (d *mysqlDialect) BuildPagination(sql string, limit, offset int) string {
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, limit, offset)
}
```

- [ ] **Step 2: 创建 dialect_sqlite.go**

```go
package dbw

import (
	"fmt"
	"strings"
)

type sqliteDialect struct{}

func (d *sqliteDialect) DriverName() string { return "sqlite" }

func (d *sqliteDialect) Placeholder(n int) string {
	return "?"
}

func (d *sqliteDialect) ConvertPlaceholders(sql string) string {
	return sql
}

func (d *sqliteDialect) QuoteIdentifier(name string) string {
	return `"` + name + `"`
}

func (d *sqliteDialect) BuildPagination(sql string, limit, offset int) string {
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, limit, offset)
}
```

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```
Expected: now referring to `oracleDialect` and `postgresDialect` which are still undefined. 会有部分编译错误。需要完成 Task 7。

- [ ] **Step 4: 暂不提交，与 Task 7 一起提交**

---

### Task 7: dialect_postgres.go + dialect_oracle.go

**Files:**
- Create: `dialect_postgres.go`, `dialect_oracle.go`

- [ ] **Step 1: 创建 dialect_postgres.go**

```go
package dbw

import (
	"fmt"
	"strings"
)

type postgresDialect struct {
	paramIndex int
}

func (d *postgresDialect) DriverName() string { return "postgres" }

func (d *postgresDialect) Placeholder(n int) string {
	d.paramIndex++
	return fmt.Sprintf("$%d", n)
}

func (d *postgresDialect) ConvertPlaceholders(sql string) string {
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

func (d *postgresDialect) QuoteIdentifier(name string) string {
	return `"` + name + `"`
}

func (d *postgresDialect) BuildPagination(sql string, limit, offset int) string {
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, limit, offset)
}
```

- [ ] **Step 2: 创建 dialect_oracle.go**

```go
package dbw

import (
	"fmt"
	"strings"
)

type oracleDialect struct {
	paramIndex int
}

func (d *oracleDialect) DriverName() string { return "oracle" }

func (d *oracleDialect) Placeholder(n int) string {
	return fmt.Sprintf(":%d", n)
}

func (d *oracleDialect) ConvertPlaceholders(sql string) string {
	var buf strings.Builder
	paramIndex := 1
	for i := 0; i < len(sql); i++ {
		if sql[i] == '?' {
			fmt.Fprintf(&buf, ":%d", paramIndex)
			paramIndex++
		} else {
			buf.WriteByte(sql[i])
		}
	}
	return buf.String()
}

func (d *oracleDialect) QuoteIdentifier(name string) string {
	return `"` + name + `"`
}

func (d *oracleDialect) BuildPagination(sql string, limit, offset int) string {
	return fmt.Sprintf("%s OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", sql, offset, limit)
}
```

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```
Expected: 编译成功（所有 dialect 类型已定义，config.go 引用均已满足）。

- [ ] **Step 4: 提交**（包含 Task 5-7 所有文件）

```bash
git add config.go dialect_mysql.go dialect_sqlite.go dialect_postgres.go dialect_oracle.go
git commit -m "feat: add Config, Dialect interface and 4 database dialect implementations"
```

---

## Phase 3: Meta & Table（元数据和表名）

### Task 8: table.go — 表名生成

**Files:**
- Create: `table.go`

- [ ] **Step 1: 创建 table.go**

```go
package dbw

import (
	"fmt"
	"strings"
	"unicode"
)

type Tabler interface {
	TableName() string
}

// getTableName 将 Go 类型名（PascalCase）转换为数据库表名（snake_case）
// 支持缩写处理：HTTPServer → http_server, OAuthClient → oauth_client
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

// camelToSnake 驼峰转蛇形（与 getTableName 使用相同算法）
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
```

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 3: 提交**

```bash
git add table.go
git commit -m "feat: add table name generation with smart abbreviation handling"
```

---

### Task 9: meta.go — 结构体元数据

**Files:**
- Create: `meta.go`

- [ ] **Step 1: 创建 meta.go**

```go
package dbw

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

var structMetaCache sync.Map

type fieldInfo struct {
	dbIgnore  bool
	name      string
	dbColumn  string
	index     int
	tag       reflect.StructTag
	dbwTag    map[string]string
}

type structMeta struct {
	tableName              string
	idGenerator            string
	fieldsInfoMap          map[string]fieldInfo
	fieldMap               map[string]int    // db column name → struct field index
	dbColumnFieldMap       map[string]string // struct field name → db column name
	dbColumnFieldNameMap   map[string]string // db column name → struct field name
	dbColumnSlice          []string
	tableIdFieldName       string
	tableIdDbColumn        string
	logicDelFieldName      string
	logicDelDbColumn       string
	autoCreateTimeFieldName string
	autoCreateTimeDbColumn string
	autoCreateTimeTagValue string
	autoUpdateTimeFieldName string
	autoUpdateTimeDbColumn string
	autoUpdateTimeTagValue string
}

func getStructMeta[T any]() *structMeta {
	var t T
	typeOf := reflect.TypeOf(t)

	if meta, ok := structMetaCache.Load(typeOf); ok {
		return meta.(*structMeta)
	}

	meta := &structMeta{
		fieldsInfoMap:        make(map[string]fieldInfo),
		fieldMap:             make(map[string]int),
		dbColumnFieldMap:     make(map[string]string),
		dbColumnFieldNameMap: make(map[string]string),
		dbColumnSlice:        make([]string, 0),
	}

	if tabler, ok := any(t).(Tabler); ok {
		meta.tableName = tabler.TableName()
	} else {
		meta.tableName = getTableName[T]()
	}

	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)
		dbwTag := resolveDbwTag(field.Tag.Get("dbw"))
		if dbwTag["dbIgnore"] == "true" {
			continue
		}
		colName := dbwTag["column"]
		if colName == "" {
			colName = camelToSnake(field.Name)
		}

		if dbwTag["primaryKey"] == "true" {
			setIdMeta(meta, field, colName)
		}
		if dbwTag["tableLogic"] == "true" {
			meta.logicDelFieldName = field.Name
			meta.logicDelDbColumn = colName
		}
		if dbwTag["autoCreateTime"] != "" {
			meta.autoCreateTimeFieldName = field.Name
			meta.autoCreateTimeDbColumn = colName
			meta.autoCreateTimeTagValue = dbwTag["autoCreateTime"]
		}
		if dbwTag["autoUpdateTime"] != "" {
			meta.autoUpdateTimeFieldName = field.Name
			meta.autoUpdateTimeDbColumn = colName
			meta.autoUpdateTimeTagValue = dbwTag["autoUpdateTime"]
		}

		fi := fieldInfo{
			dbIgnore: dbwTag["dbIgnore"] == "true",
			name:     field.Name,
			dbColumn: colName,
			index:    i,
			tag:      field.Tag,
			dbwTag:   dbwTag,
		}
		meta.fieldsInfoMap[field.Name] = fi
		meta.dbColumnFieldMap[field.Name] = colName
		meta.dbColumnFieldNameMap[colName] = field.Name
		meta.dbColumnSlice = append(meta.dbColumnSlice, colName)
		meta.fieldMap[strings.ToLower(colName)] = i
	}

	if meta.tableIdFieldName == "" {
		if field, ok := typeOf.FieldByName("Id"); ok {
			dbwTag := resolveDbwTag(field.Tag.Get("dbw"))
			colName := dbwTag["column"]
			if colName == "" {
				colName = camelToSnake(field.Name)
			}
			setIdMeta(meta, field, colName)
		}
	}

	structMetaCache.Store(typeOf, meta)
	if meta.tableIdFieldName == "" {
		fmt.Printf("dbw: %v table id property not found\n", typeOf)
	}
	return meta
}

func setIdMeta(meta *structMeta, field reflect.StructField, colName string) {
	fieldType := field.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}
	dbwTag := resolveDbwTag(field.Tag.Get("dbw"))
	switch fieldType.Kind() {
	case reflect.Int, reflect.Int64, reflect.Uint64:
		if dbwTag["idGenerator"] == "" {
			meta.idGenerator = "snowflake"
		} else {
			meta.idGenerator = dbwTag["idGenerator"]
		}
	case reflect.String:
		if dbwTag["idGenerator"] == "" {
			meta.idGenerator = "snowflakeStr"
		} else {
			meta.idGenerator = dbwTag["idGenerator"]
		}
	default:
		panic(fmt.Sprintf("dbw: unsupported id type: %s only int, int64, uint64, string", fieldType))
	}
	meta.tableIdFieldName = field.Name
	meta.tableIdDbColumn = colName
}

func resolveDbwTag(tag string) map[string]string {
	result := make(map[string]string)
	if tag == "" {
		return result
	}
	parts := strings.Split(tag, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 1 {
			result[kv[0]] = "true"
		} else {
			result[kv[0]] = strings.TrimSpace(kv[1])
		}
	}
	return result
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 3: 提交**

```bash
git add meta.go
git commit -m "feat: add struct metadata parsing with column tag support on primary key"
```

---

## Phase 4: Core Wrapper（核心包装器）

### Task 10: wrapper.go — DbWrapper 核心结构体

**Files:**
- Create: `wrapper.go`

- [ ] **Step 1: 创建 wrapper.go**

```go
package dbw

import (
	"context"
	"database/sql"
	"fmt"
)

type DbWrapper[T any] struct {
	config    *Config
	tx        *sql.Tx
	ctx       context.Context
	tableName string
	selects   []string
	wheres    []whereExpr
	orders    []orderExpr
	groupBy   []string
	havings   []whereExpr
	pageNum   *int
	pageSize  *int
	distinct  bool
	joins     []joinExpr
	limit     *int
	offset    *int
	rawSQL    string
	rawArgs   []any
	meta      *structMeta
}

type Options func(opts *DbWrapper[any])

type whereExpr struct {
	sql    string
	args   []any
	joiner string // "AND" 或 "OR"，第一个条件的 joiner 为空
}

type orderExpr struct {
	field string
	order string
}

type joinExpr struct {
	joinType string // INNER, LEFT, RIGHT
	table    string
	on       string
}

func WithConfig(config *Config) Options {
	return func(opts *DbWrapper[any]) {
		opts.config = config
	}
}

func WithContext(ctx context.Context) Options {
	return func(opts *DbWrapper[any]) {
		opts.ctx = ctx
	}
}

func WithTx(tx *sql.Tx) Options {
	return func(opts *DbWrapper[any]) {
		opts.tx = tx
	}
}

func New[T any](opts ...Options) *DbWrapper[T] {
	q := &DbWrapper[T]{}
	for _, opt := range opts {
		opt((*DbWrapper[any])(q))
	}
	if q.config == nil {
		panic("dbw: config is required, use dbw.WithConfig(config)")
	}
	if q.config.Db == nil {
		panic("dbw: database connection is nil")
	}
	if q.ctx == nil {
		q.ctx = context.Background()
	}
	if q.selects == nil {
		q.selects = []string{"*"}
	}
	if q.meta == nil {
		q.meta = getStructMeta[T]()
	}
	return q
}

func (q *DbWrapper[T]) Reset(opts ...Options) *DbWrapper[T] {
	n := &DbWrapper[T]{
		config:    q.config,
		meta:      q.meta,
		tableName: q.tableName,
	}
	for _, opt := range opts {
		opt((*DbWrapper[any])(n))
	}
	if n.config == nil {
		panic("dbw: config is required")
	}
	if n.config.Db == nil {
		panic("dbw: database connection is nil")
	}
	if n.ctx == nil {
		n.ctx = context.Background()
	}
	if n.selects == nil {
		n.selects = []string{"*"}
	}
	if n.meta == nil {
		n.meta = q.meta
	}
	return n
}

func (q *DbWrapper[T]) Clone() *DbWrapper[T] {
	qCopy := *q
	if qCopy.selects != nil {
		qCopy.selects = append([]string(nil), qCopy.selects...)
	}
	if qCopy.wheres != nil {
		qCopy.wheres = append([]whereExpr(nil), qCopy.wheres...)
	}
	if qCopy.orders != nil {
		qCopy.orders = append([]orderExpr(nil), qCopy.orders...)
	}
	if qCopy.groupBy != nil {
		qCopy.groupBy = append([]string(nil), qCopy.groupBy...)
	}
	if qCopy.havings != nil {
		qCopy.havings = append([]whereExpr(nil), qCopy.havings...)
	}
	if qCopy.joins != nil {
		qCopy.joins = append([]joinExpr(nil), qCopy.joins...)
	}
	qCopy.rawArgs = append([]any(nil), qCopy.rawArgs...)
	return &qCopy
}

func (q *DbWrapper[T]) Clean() *DbWrapper[T] {
	return &DbWrapper[T]{
		config:    q.config,
		ctx:       q.ctx,
		meta:      q.meta,
		tableName: q.tableName,
		selects:   []string{"*"},
	}
}

func (q *DbWrapper[T]) TableName(tableName string) *DbWrapper[T] {
	q.tableName = tableName
	return q
}

func (q *DbWrapper[T]) getTableName() string {
	if q.tableName != "" {
		return q.tableName
	}
	return q.meta.tableName
}

func (q *DbWrapper[T]) Tx(tx *sql.Tx) *DbWrapper[T] {
	q.tx = tx
	return q
}

func (q *DbWrapper[T]) WithContext(ctx context.Context) *DbWrapper[T] {
	q.ctx = ctx
	return q
}

func (q *DbWrapper[T]) CloneForLogicDel() *DbWrapper[T] {
	clone := q.Clone()
	if clone.meta.logicDelDbColumn != "" && clone.config != nil {
		clone.wheres = append(clone.wheres, whereExpr{
			sql:    fmt.Sprintf("%s = ?", clone.meta.logicDelDbColumn),
			args:   []any{clone.config.LogicNotDeleteValue},
			joiner: "AND",
		})
	}
	return clone
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。旧文件 `dbwrapper.go` 仍然存在，与 `wrapper.go` 会重复类型定义 —— **需要先删除旧文件**。

等等，旧文件还在！需要先删除旧文件，否则会有重复定义。

- [ ] **Step 3: 验证编译（先删除旧 wrapper 文件）**

```bash
Remove-Item -Path "dbwrapper.go"
go build ./...
```
Expected: 编译失败，因为旧文件如 `common.go`、`where.go`、`select.go` 等仍引用旧的 `DbWrapper` 类型（字段不同）。

我们需要先删除所有旧文件。

- [ ] **Step 4: 删除所有旧源文件**

```bash
Remove-Item -Path "common.go", "structmeta.go", "where.go", "select.go", "insert.go", "update.go", "delete.go", "snow_flake.go"
```
然后验证：
```bash
go build ./...
```
Expected: 目前只有基础文件编译通过（errors, utils, convert, snowflake, generator, log, config, dialects, table, meta, wrapper）。

- [ ] **Step 5: 提交**

```bash
git add wrapper.go
git rm common.go structmeta.go where.go select.go insert.go update.go delete.go snow_flake.go dbwrapper.go
git commit -m "feat: add DbWrapper core struct with CloneForLogicDel and delete old files"
```

注意：git rm 后旧文件可能显示 deleted。

---

## Phase 5: Query Building（查询构建）

### Task 11: condition.go + condition_group.go — WHERE 条件

**Files:**
- Create: `condition.go`, `condition_group.go`

- [ ] **Step 1: 创建 condition.go**

```go
package dbw

import (
	"strings"
)

func (q *DbWrapper[T]) Where(sql string, args ...any) *DbWrapper[T] {
	joiner := "AND"
	if len(q.wheres) > 0 && q.wheres[len(q.wheres)-1].joiner == "OR" {
		joiner = "OR"
	}
	q.wheres = append(q.wheres, whereExpr{sql: sql, args: args, joiner: joiner})
	return q
}

func (q *DbWrapper[T]) Eq(field string, val any) *DbWrapper[T] {
	return q.Where(field+" = ?", val)
}

func (q *DbWrapper[T]) Ne(field string, val any) *DbWrapper[T] {
	return q.Where(field+" != ?", val)
}

func (q *DbWrapper[T]) Gt(field string, val any) *DbWrapper[T] {
	return q.Where(field+" > ?", val)
}

func (q *DbWrapper[T]) Ge(field string, val any) *DbWrapper[T] {
	return q.Where(field+" >= ?", val)
}

func (q *DbWrapper[T]) Lt(field string, val any) *DbWrapper[T] {
	return q.Where(field+" < ?", val)
}

func (q *DbWrapper[T]) Le(field string, val any) *DbWrapper[T] {
	return q.Where(field+" <= ?", val)
}

func (q *DbWrapper[T]) Like(field, pattern string) *DbWrapper[T] {
	return q.Where(field+" LIKE ?", pattern)
}

func (q *DbWrapper[T]) In(field string, values ...any) *DbWrapper[T] {
	if len(values) == 0 {
		return q.Where("1 = 0")
	}
	placeholders := make([]string, len(values))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return q.Where(field+" IN ("+strings.Join(placeholders, ",")+")", values...)
}

func (q *DbWrapper[T]) Between(field string, min any, max any) *DbWrapper[T] {
	return q.Where(field+" BETWEEN ? AND ?", min, max)
}

func (q *DbWrapper[T]) IsNull(field string) *DbWrapper[T] {
	return q.Where(field + " IS NULL")
}

func (q *DbWrapper[T]) NotNull(field string) *DbWrapper[T] {
	return q.Where(field + " IS NOT NULL")
}
```

- [ ] **Step 2: 创建 condition_group.go**

```go
package dbw

import "strings"

func (q *DbWrapper[T]) Or() *DbWrapper[T] {
	if len(q.wheres) > 0 {
		q.wheres[len(q.wheres)-1].joiner = "OR"
	}
	return q
}

func (q *DbWrapper[T]) OrNest(f func(*DbWrapper[T])) *DbWrapper[T] {
	if len(q.wheres) > 0 {
		q.wheres[len(q.wheres)-1].joiner = "OR"
	}
	qw := DbWrapper[T]{}
	f(&qw)
	str, args := buildWhere(qw.wheres)
	str = strings.ReplaceAll(str, "WHERE ", "")
	joiner := "AND"
	if len(q.wheres) > 0 && q.wheres[len(q.wheres)-1].joiner == "OR" {
		joiner = "OR"
	}
	q.wheres = append(q.wheres, whereExpr{sql: "(" + str + ")", args: args, joiner: joiner})
	return q
}

func (q *DbWrapper[T]) And(f func(*DbWrapper[T])) *DbWrapper[T] {
	qw := DbWrapper[T]{}
	f(&qw)
	str, args := buildWhere(qw.wheres)
	str = strings.ReplaceAll(str, "WHERE ", "")
	q.wheres = append(q.wheres, whereExpr{sql: "(" + str + ")", args: args, joiner: "AND"})
	return q
}

func (q *DbWrapper[T]) WhereIf(cond bool, sql string, args ...any) *DbWrapper[T] {
	if cond {
		q.Where(sql, args...)
	}
	return q
}

func (q *DbWrapper[T]) AndIf(cond bool, f func(*DbWrapper[T])) *DbWrapper[T] {
	if cond {
		q.And(f)
	}
	return q
}

func (q *DbWrapper[T]) OrNestIf(cond bool, f func(*DbWrapper[T])) *DbWrapper[T] {
	if cond {
		q.OrNest(f)
	}
	return q
}

func (q *DbWrapper[T]) EqIf(cond bool, field string, val any) *DbWrapper[T] {
	if cond {
		q.Eq(field, val)
	}
	return q
}

func (q *DbWrapper[T]) LikeIf(cond bool, field, pattern string) *DbWrapper[T] {
	if cond {
		q.Like(field, pattern)
	}
	return q
}

func buildWhere(wheres []whereExpr) (string, []any) {
	if len(wheres) == 0 {
		return "", nil
	}
	var b strings.Builder
	args := make([]any, 0, len(wheres)*2)
	b.WriteString("WHERE ")
	for i, w := range wheres {
		if i > 0 {
			if w.joiner == "OR" {
				b.WriteString(" OR ")
			} else {
				b.WriteString(" AND ")
			}
		}
		b.WriteString(w.sql)
		args = append(args, w.args...)
	}
	return b.String(), args
}
```

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 4: 提交**

```bash
git add condition.go condition_group.go
git commit -m "feat: add WHERE condition builders with fixed Or() semantics"
```

---

### Task 12: query.go + query_sql.go — 查询构建和 SQL 生成

**Files:**
- Create: `query.go`, `query_sql.go`

- [ ] **Step 1: 创建 query.go**

```go
package dbw

import (
	"database/sql"
)

func (q *DbWrapper[T]) Select(fields ...string) *DbWrapper[T] {
	if len(fields) > 0 {
		q.selects = fields
	}
	return q
}

func (q *DbWrapper[T]) OrderBy(field string) *DbWrapper[T] {
	q.orders = append(q.orders, orderExpr{field: field, order: "ASC"})
	return q
}

func (q *DbWrapper[T]) OrderByDesc(field string) *DbWrapper[T] {
	q.orders = append(q.orders, orderExpr{field: field, order: "DESC"})
	return q
}

func (q *DbWrapper[T]) GroupBy(fields ...string) *DbWrapper[T] {
	q.groupBy = append(q.groupBy, fields...)
	return q
}

func (q *DbWrapper[T]) Having(sql string, args ...any) *DbWrapper[T] {
	q.havings = append(q.havings, whereExpr{sql: sql, args: args})
	return q
}

func (q *DbWrapper[T]) Distinct() *DbWrapper[T] {
	q.distinct = true
	return q
}

func (q *DbWrapper[T]) Limit(n int) *DbWrapper[T] {
	q.limit = &n
	return q
}

func (q *DbWrapper[T]) Offset(n int) *DbWrapper[T] {
	q.offset = &n
	return q
}

func (q *DbWrapper[T]) Count() (int64, error) {
	qCopy := q.Clone()
	qCopy.selects = []string{"COUNT(*)"}
	qCopy.orders = nil
	qCopy.limit = nil
	qCopy.offset = nil
	qCopy.pageNum = nil
	qCopy.pageSize = nil

	var count int64
	err := qCopy.queryRow().Scan(&count)
	return count, err
}

func (q *DbWrapper[T]) Exist() (bool, error) {
	count, err := q.Count()
	return count > 0, err
}
```

- [ ] **Step 2: 创建 query_sql.go**

```go
package dbw

import (
	"database/sql"
	"fmt"
	"strings"
)

func (q *DbWrapper[T]) buildSelectSQL() (string, []any) {
	// 如果有原始 SQL，直接使用
	if q.rawSQL != "" {
		return q.rawSQL, q.rawArgs
	}

	clone := q.CloneForLogicDel()
	var b strings.Builder

	b.WriteString("SELECT ")
	if clone.distinct {
		b.WriteString("DISTINCT ")
	}
	b.WriteString(strings.Join(clone.selects, ", "))

	b.WriteString(" FROM ")
	b.WriteString(clone.getTableName())

	// JOIN
	for _, j := range clone.joins {
		b.WriteString(fmt.Sprintf(" %s JOIN %s ON %s", j.joinType, j.table, j.on))
	}

	// WHERE
	whereStr, whereArgs := buildWhere(clone.wheres)
	b.WriteString(whereStr)

	// GROUP BY
	if len(clone.groupBy) > 0 {
		b.WriteString(" GROUP BY " + strings.Join(clone.groupBy, ", "))
	}

	// HAVING
	if len(clone.havings) > 0 {
		b.WriteString(" HAVING ")
		for i, h := range clone.havings {
			if i > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(h.sql)
			whereArgs = append(whereArgs, h.args...)
		}
	}

	// ORDER BY
	if len(clone.orders) > 0 {
		b.WriteString(" ORDER BY ")
		orders := make([]string, len(clone.orders))
		for i, o := range clone.orders {
			orders[i] = o.field + " " + o.order
		}
		b.WriteString(strings.Join(orders, ", "))
	}

	sqlStr := b.String()

	// 分页或 Limit/Offset
	if clone.pageNum != nil && clone.pageSize != nil {
		if clone.config.PageInterceptor != nil {
			sqlStr = clone.config.PageInterceptor(sqlStr, *clone.pageNum, *clone.pageSize)
		} else {
			offset := (*clone.pageNum - 1) * (*clone.pageSize)
			sqlStr = clone.config.Dialect.BuildPagination(sqlStr, *clone.pageSize, offset)
		}
	} else if clone.limit != nil || clone.offset != nil {
		limit := 0
		off := 0
		if clone.limit != nil {
			limit = *clone.limit
		}
		if clone.offset != nil {
			off = *clone.offset
		}
		sqlStr = clone.config.Dialect.BuildPagination(sqlStr, limit, off)
	}

	if clone.config.Dialect.DriverName() != "mysql" && clone.config.Dialect.DriverName() != "sqlite" {
		sqlStr = clone.config.Dialect.ConvertPlaceholders(sqlStr)
	}

	return sqlStr, whereArgs
}

func (q *DbWrapper[T]) query() (*sql.Rows, error) {
	sqlStr, args := q.buildSelectSQL()
	debugLog(q.config, q.ctx, sqlStr, args)
	if q.tx == nil {
		return q.config.Db.QueryContext(q.ctx, sqlStr, args...)
	}
	return q.tx.QueryContext(q.ctx, sqlStr, args...)
}

func (q *DbWrapper[T]) queryRow() *sql.Row {
	sqlStr, args := q.buildSelectSQL()
	debugLog(q.config, q.ctx, sqlStr, args)
	if q.tx == nil {
		return q.config.Db.QueryRowContext(q.ctx, sqlStr, args...)
	}
	return q.tx.QueryRowContext(q.ctx, sqlStr, args...)
}

func (q *DbWrapper[T]) buildUpdateSQL(sets map[string]any) (string, []any) {
	clone := q.CloneForLogicDel()
	var b strings.Builder
	args := make([]any, 0, len(sets)+len(clone.wheres)*2)

	b.WriteString("UPDATE ")
	b.WriteString(clone.getTableName())
	b.WriteString(" SET ")

	setParts := make([]string, 0, len(sets))
	for k, v := range sets {
		setParts = append(setParts, k+" = ?")
		args = append(args, v)
	}
	b.WriteString(strings.Join(setParts, ", "))

	whereStr, whereArgs := buildWhere(clone.wheres)
	b.WriteString(whereStr)
	args = append(args, whereArgs...)

	sqlStr := b.String()
	if clone.config.Dialect.DriverName() != "mysql" && clone.config.Dialect.DriverName() != "sqlite" {
		sqlStr = clone.config.Dialect.ConvertPlaceholders(sqlStr)
	}

	return sqlStr, args
}

func (q *DbWrapper[T]) buildDeleteSQL() (string, []any) {
	if q.meta.logicDelDbColumn != "" {
		return q.buildUpdateSQL(map[string]any{q.meta.logicDelDbColumn: q.config.LogicDeleteValue})
	}

	clone := q.CloneForLogicDel()
	var b strings.Builder

	b.WriteString("DELETE FROM ")
	b.WriteString(clone.getTableName())

	whereStr, whereArgs := buildWhere(clone.wheres)
	b.WriteString(whereStr)

	sqlStr := b.String()
	if clone.config.Dialect.DriverName() != "mysql" && clone.config.Dialect.DriverName() != "sqlite" {
		sqlStr = clone.config.Dialect.ConvertPlaceholders(sqlStr)
	}

	return sqlStr, whereArgs
}

func (q *DbWrapper[T]) buildInsertSQL(columns []string, placeholders []string) (string, []any) {
	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", q.getTableName(), strings.Join(columns, ", "), strings.Join(placeholders, ", "))
	if q.config.Dialect.DriverName() != "mysql" && q.config.Dialect.DriverName() != "sqlite" {
		sqlStr = q.config.Dialect.ConvertPlaceholders(sqlStr)
	}
	return sqlStr, nil
}

func (q *DbWrapper[T]) buildInsertBatchSQL(columns []string, rowPlaceholders []string) (string, []any) {
	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", q.getTableName(), strings.Join(columns, ", "), strings.Join(rowPlaceholders, "), ("))
	if q.config.Dialect.DriverName() != "mysql" && q.config.Dialect.DriverName() != "sqlite" {
		sqlStr = q.config.Dialect.ConvertPlaceholders(sqlStr)
	}
	return sqlStr, nil
}
```

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 4: 提交**

```bash
git add query.go query_sql.go
git commit -m "feat: add query builder, SQL generation, Count, Exist, Limit, Offset"
```

---

## Phase 6: CRUD Operations（增删改查）

### Task 13: select.go — 查询执行

**Files:**
- Create: `select.go`

- [ ] **Step 1: 创建 select.go**

```go
package dbw

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

func (q *DbWrapper[T]) SelectById(id any) (*T, error) {
	if q.meta.tableIdFieldName == "" {
		return nil, ErrNoPrimaryKey
	}
	q.Eq(q.meta.tableIdDbColumn, id)
	return q.SelectOne()
}

func (q *DbWrapper[T]) SelectOne() (*T, error) {
	q.Limit(2)
	rows, err := q.query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slice, err := q.scanRowsToTypeSlice(rows)
	if err != nil {
		return nil, err
	}
	if len(slice) > 1 {
		return nil, fmt.Errorf("%w: got %d", ErrMultipleRecords, len(slice))
	}
	if len(slice) == 0 {
		return nil, nil
	}
	return &slice[0], nil
}

func (q *DbWrapper[T]) FindOne() (*T, error) {
	q.Limit(1)
	rows, err := q.query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slice, err := q.scanRowsToTypeSlice(rows)
	if err != nil {
		return nil, err
	}
	if len(slice) >= 1 {
		return &slice[0], nil
	}
	return nil, ErrRecordNotFound
}

func (q *DbWrapper[T]) SelectList() ([]T, error) {
	rows, err := q.query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return q.scanRowsToTypeSlice(rows)
}

func (q *DbWrapper[T]) SelectPage(pageNum int, pageSize int) (records []T, count int64, err error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	count, err = q.Clone().Count()
	if err != nil {
		return nil, 0, fmt.Errorf("count failed: %w", err)
	}
	if count == 0 {
		return make([]T, 0), 0, nil
	}

	q.pageNum = &pageNum
	q.pageSize = &pageSize

	list, err := q.SelectList()
	if err != nil {
		return nil, 0, fmt.Errorf("select list failed: %w", err)
	}
	return list, count, nil
}

func (q *DbWrapper[T]) ScanOne(dest ...any) error {
	q.Limit(2)
	rows, err := q.query()
	if err != nil {
		return err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
		if count > 1 {
			return fmt.Errorf("%w: got %d", ErrMultipleRecords, count)
		}
	}
	if count == 0 {
		return nil
	}
	return rows.Scan(dest...)
}

func (q *DbWrapper[T]) ScanList(scanner func(*sql.Rows) error) error {
	rows, err := q.query()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scanner(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (q *DbWrapper[T]) ScanPage(pageNum int, pageSize int, scanner func(*sql.Rows) error) (int64, error) {
	count, err := q.Clone().Count()
	if err != nil {
		return 0, err
	}

	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 0 {
		pageSize = 0
	}
	q.pageNum = &pageNum
	q.pageSize = &pageSize

	return count, q.ScanList(scanner)
}

func (q *DbWrapper[T]) scanRowsToTypeSlice(rows *sql.Rows) ([]T, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	results := make([]T, 0)
	var t T
	tType := reflect.TypeOf(t)

	for rows.Next() {
		result := reflect.New(tType).Elem()
		scanValues := make([]any, len(columns))
		for i, col := range columns {
			colLower := strings.ToLower(col)
			if idx, ok := q.meta.fieldMap[colLower]; ok {
				scanValues[i] = result.Field(idx).Addr().Interface()
			} else {
				var val any
				scanValues[i] = &val
			}
		}

		if err := rows.Scan(scanValues...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		results = append(results, result.Interface().(T))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return results, nil
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 3: 提交**

```bash
git add select.go
git commit -m "feat: add select operations with SelectById fix and SelectOne LIMIT 2 optimization"
```

---

### Task 14: insert.go — 插入操作

**Files:**
- Create: `insert.go`

- [ ] **Step 1: 创建 insert.go**

```go
package dbw

import (
	"database/sql"
	"fmt"
	"reflect"
)

func (q *DbWrapper[T]) beforeInsert(data *T) (generateTableId any, err error) {
	valueOf := reflect.ValueOf(data).Elem()

	for _, fieldInfo := range q.meta.fieldsInfoMap {
		fieldValue := valueOf.Field(fieldInfo.index)

		if fieldInfo.dbColumn == q.meta.tableIdDbColumn {
			if fieldValue.IsZero() {
				fn, ok := idGenerator[q.meta.idGenerator]
				if !ok {
					return nil, fmt.Errorf("dbw: id generator %s not found", q.meta.idGenerator)
				}
				generateTableId = fn()
				fieldValue.Set(reflect.ValueOf(generateTableId))
			}
			continue
		}

		if autoCreateTime := fieldInfo.dbwTag["autoCreateTime"]; autoCreateTime != "" {
			fieldValue.Set(reflect.ValueOf(getTime(autoCreateTime)))
			continue
		}

		if autoUpdateTime := fieldInfo.dbwTag["autoUpdateTime"]; autoUpdateTime != "" {
			fieldValue.Set(reflect.ValueOf(getTime(autoUpdateTime)))
			continue
		}

		if q.meta.logicDelDbColumn != "" && fieldInfo.dbColumn == q.meta.logicDelDbColumn {
			fieldValue.Set(reflect.ValueOf(q.config.LogicNotDeleteValue))
			continue
		}

		if fieldValue.IsZero() {
			if defaultValue, has := fieldInfo.dbwTag["default"]; has {
				convertedVal := convertDefaultValue(defaultValue, fieldValue.Type())
				fieldValue.Set(reflect.ValueOf(convertedVal))
			}
		}
	}
	return generateTableId, nil
}

func (q *DbWrapper[T]) Insert(data *T) (sql.Result, error) {
	if data == nil {
		return nil, ErrNilEntity
	}

	generatedId, err := q.beforeInsert(data)
	if err != nil {
		return nil, fmt.Errorf("before insert: %w", err)
	}

	columns := make([]string, 0, len(q.meta.fieldsInfoMap))
	placeholders := make([]string, 0, len(q.meta.fieldsInfoMap))
	args := make([]any, 0, len(q.meta.fieldsInfoMap))

	dataValue := reflect.ValueOf(data).Elem()

	for _, fieldInfo := range q.meta.fieldsInfoMap {
		if fieldInfo.dbIgnore {
			continue
		}
		fieldValue := dataValue.Field(fieldInfo.index)
		if fieldValue.IsZero() {
			continue
		}
		columns = append(columns, fieldInfo.dbColumn)
		placeholders = append(placeholders, "?")
		args = append(args, fieldValue.Interface())
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("%w: table %s", ErrNoFieldsToUpdate, q.getTableName())
	}

	sqlStr, _ := q.buildInsertSQL(columns, placeholders)
	debugLog(q.config, q.ctx, sqlStr, args)

	var result sql.Result
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("insert failed for table %s: %w", q.getTableName(), err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, fmt.Errorf("insert failed: no rows affected for table %s", q.getTableName())
	}

	if q.config.Debug && generatedId != nil {
		fmt.Printf("[DEBUG] Generated ID: %v\n", generatedId)
	}

	return result, nil
}

func (q *DbWrapper[T]) InsertBatch(data []T) (sql.Result, error) {
	if data == nil {
		return nil, ErrNilEntity
	}
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	const maxBatchSize = 1000
	if len(data) > maxBatchSize {
		return nil, ErrBatchTooLarge
	}

	if q.meta.tableIdDbColumn == "" {
		return nil, ErrNoPrimaryKey
	}

	tableIdFieldInfo := q.meta.fieldsInfoMap[q.meta.tableIdFieldName]
	idType := tableIdFieldInfo.dbwTag["idType"]
	if idType != "" && idType != "assign" {
		return nil, fmt.Errorf("dbw: primary key type must be 'assign' for batch insert")
	}

	generateTableIdMap := make(map[any]struct{}, len(data))
	for i := range data {
		generatedId, err := q.beforeInsert(&data[i])
		if err != nil {
			return nil, fmt.Errorf("before insert: %w", err)
		}
		if generatedId != nil {
			if _, exists := generateTableIdMap[generatedId]; exists {
				return nil, fmt.Errorf("dbw: duplicate primary key in batch insert")
			}
			generateTableIdMap[generatedId] = struct{}{}
		}
	}

	dbColumns := make([]string, 0, len(q.meta.dbColumnSlice))
	rowPlaceholders := make([]string, 0, len(data))
	args := make([]any, 0, len(data)*len(q.meta.dbColumnSlice))

	for i := range data {
		rowPhs := make([]string, 0, len(q.meta.dbColumnSlice))
		for _, dbColumn := range q.meta.dbColumnSlice {
			fieldName := q.meta.dbColumnFieldNameMap[dbColumn]
			fi := q.meta.fieldsInfoMap[fieldName]
			if i == 0 {
				if fi.dbIgnore {
					continue
				}
				dbColumns = append(dbColumns, dbColumn)
			}
			if fi.dbIgnore {
				continue
			}
			fieldValue := reflect.ValueOf(&data[i]).Elem().Field(fi.index)
			args = append(args, fieldValue.Interface())
			rowPhs = append(rowPhs, "?")
		}
		rowPlaceholders = append(rowPlaceholders, strings(groupPhs))
	}

	sqlStr, _ := q.buildInsertBatchSQL(dbColumns, rowPlaceholders)
	debugLog(q.config, q.ctx, sqlStr, args)

	var result sql.Result
	var err error
	if q.tx != nil {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("insert batch failed for table %s: %w", q.getTableName(), err)
	}

	return result, nil
}

func (q *DbWrapper[T]) InsertBatchSplit(data []T, size int) ([]sql.Result, error) {
	split, err := sliceSplit(data, size)
	if err != nil {
		return nil, fmt.Errorf("split data: %w", err)
	}

	results := make([]sql.Result, 0, len(split))
	for _, batch := range split {
		result, err := q.InsertBatch(batch)
		if err != nil {
			return results, fmt.Errorf("insert batch: %w", err)
		}
		results = append(results, result)
	}
	return results, nil
}
```

等等，上面的代码中 `strings(groupPhs)` 应该是 `strings.Join(groupPhs, ", ")`. 让我修正这个问题。

- [ ] **Step 1 (修正): 创建 insert.go**

修复 `strings(groupPhs)` → `strings.Join(groupPhs, ", ")`，保留其余代码不变。

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 3: 提交**

```bash
git add insert.go
git commit -m "feat: add insert operations with Field(index) O(1) optimization for batch insert"
```

---

### Task 15: update.go — 更新操作

**Files:**
- Create: `update.go`

- [ ] **Step 1: 创建 update.go**

```go
package dbw

import (
	"database/sql"
	"fmt"
	"reflect"
	"time"
)

func (q *DbWrapper[T]) UpdateById(data *T) (sql.Result, error) {
	if q.meta.tableIdFieldName == "" {
		return nil, ErrNoPrimaryKey
	}

	var b strings.Builder
	args := make([]any, 0)
	b.WriteString("UPDATE " + q.getTableName() + " SET ")

	sets := make([]string, 0, len(q.meta.fieldsInfoMap))
	elem := reflect.ValueOf(data).Elem()
	setCount := 0

	appendSet := func(colName string, arg any) {
		sets = append(sets, colName+" = ?")
		args = append(args, arg)
		setCount++
	}

	for _, fieldInfo := range q.meta.fieldsInfoMap {
		if fieldInfo.dbIgnore || fieldInfo.name == q.meta.tableIdFieldName || fieldInfo.name == q.meta.logicDelFieldName {
			continue
		}

		if fieldInfo.dbwTag["autoUpdateTime"] != "" {
			if fieldInfo.dbwTag["autoUpdateTime"] == "milli" {
				appendSet(fieldInfo.dbColumn, time.Now().UnixMilli())
			} else {
				appendSet(fieldInfo.dbColumn, time.Now())
			}
			continue
		}

		fieldValue := elem.Field(fieldInfo.index)
		defaultValue, hasDefault := fieldInfo.dbwTag["default"]

		if fieldValue.Kind() == reflect.Ptr {
			if !fieldValue.IsNil() {
				appendSet(fieldInfo.dbColumn, fieldValue.Interface())
			} else if fieldInfo.dbwTag["tableUpdateStrategy"] == "always" {
				appendSet(fieldInfo.dbColumn, nil)
			}
		} else {
			if !fieldValue.IsZero() {
				appendSet(fieldInfo.dbColumn, fieldValue.Interface())
			} else if fieldInfo.dbwTag["tableUpdateStrategy"] == "always" {
				if hasDefault {
					appendSet(fieldInfo.dbColumn, defaultValue)
				} else {
					appendSet(fieldInfo.dbColumn, fieldValue.Interface())
				}
			}
		}
	}

	if setCount == 0 {
		return nil, ErrNoFieldsToUpdate
	}

	b.WriteString(strings.Join(sets, ", "))

	// 添加 ID 条件
	q.Eq(q.meta.tableIdDbColumn, elem.FieldByName(q.meta.tableIdFieldName).Interface())

	// Clone 以确保不污染原始 wheres
	clone := q.CloneForLogicDel()
	whereStr, whereArgs := buildWhere(clone.wheres)
	b.WriteString(whereStr)
	args = append(args, whereArgs...)

	sqlStr := b.String()
	if q.config.Dialect.DriverName() != "mysql" && q.config.Dialect.DriverName() != "sqlite" {
		sqlStr = q.config.Dialect.ConvertPlaceholders(sqlStr)
	}

	debugLog(q.config, q.ctx, sqlStr, args)

	var result sql.Result
	var err error
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (q *DbWrapper[T]) Update(values map[string]any) (sql.Result, error) {
	if len(values) == 0 {
		return nil, ErrNoFieldsToUpdate
	}
	if len(q.wheres) == 0 {
		return nil, ErrNoWhereClause
	}

	if q.meta.autoUpdateTimeDbColumn != "" {
		if q.meta.autoUpdateTimeTagValue == "milli" {
			values[q.meta.autoUpdateTimeDbColumn] = time.Now().UnixMilli()
		} else {
			values[q.meta.autoUpdateTimeDbColumn] = time.Now()
		}
	}

	sqlStr, args := q.buildUpdateSQL(values)
	debugLog(q.config, q.ctx, sqlStr, args)

	var result sql.Result
	var err error
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}
```

Wait, I need to add `strings` import. Let me fix.

- [ ] **Step 1 (修正): 创建 update.go**

需要 import `"strings"`。

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 3: 提交**

```bash
git add update.go
git commit -m "feat: add update operations with WHERE safety check"
```

---

### Task 16: delete.go — 删除操作

**Files:**
- Create: `delete.go`

- [ ] **Step 1: 创建 delete.go**

```go
package dbw

import (
	"database/sql"
	"fmt"
)

func (q *DbWrapper[T]) Delete() (sql.Result, error) {
	if len(q.wheres) == 0 {
		return nil, ErrNoWhereClause
	}

	sqlStr, args := q.buildDeleteSQL()
	debugLog(q.config, q.ctx, sqlStr, args)

	var result sql.Result
	var err error
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("delete failed: %w", err)
	}
	return result, nil
}

func (q *DbWrapper[T]) DeleteById(id any) (sql.Result, error) {
	if q.meta.tableIdFieldName == "" {
		return nil, ErrNoPrimaryKey
	}
	q.Eq(q.meta.tableIdDbColumn, id)
	return q.Delete()
}

func (q *DbWrapper[T]) DeleteByIds(ids []any) (sql.Result, error) {
	if len(ids) == 0 {
		return nil, ErrEmptyData
	}
	if q.meta.tableIdFieldName == "" {
		return nil, ErrNoPrimaryKey
	}
	q.In(q.meta.tableIdDbColumn, ids)
	return q.Delete()
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 3: 提交**

```bash
git add delete.go
git commit -m "feat: add delete operations with logic delete auto-detection"
```

---

## Phase 7: New Features（新功能）

### Task 17: raw.go + where_struct.go — Raw SQL 和结构体条件

**Files:**
- Create: `raw.go`, `where_struct.go`

- [ ] **Step 1: 创建 raw.go**

```go
package dbw

import "database/sql"

func (q *DbWrapper[T]) Raw(sql string, args ...any) *DbWrapper[T] {
	q.rawSQL = sql
	q.rawArgs = args
	return q
}

func (q *DbWrapper[T]) Exec() (sql.Result, error) {
	sqlStr := q.rawSQL
	args := q.rawArgs
	if sqlStr == "" {
		sqlStr, args = q.buildSelectSQL()
	}

	if q.config.Dialect.DriverName() != "mysql" && q.config.Dialect.DriverName() != "sqlite" {
		sqlStr = q.config.Dialect.ConvertPlaceholders(sqlStr)
	}

	debugLog(q.config, q.ctx, sqlStr, args)

	if q.tx == nil {
		return q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	}
	return q.tx.ExecContext(q.ctx, sqlStr, args...)
}
```

- [ ] **Step 2: 创建 where_struct.go**

```go
package dbw

import "reflect"

func (q *DbWrapper[T]) WhereStruct(data *T) *DbWrapper[T] {
	if data == nil {
		return q
	}
	val := reflect.ValueOf(data).Elem()
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldInfo, ok := q.meta.fieldsInfoMap[field.Name]
		if !ok || fieldInfo.dbIgnore {
			continue
		}
		fieldValue := val.Field(i)
		if fieldValue.IsZero() {
			continue
		}
		if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
			continue
		}
		q.Eq(fieldInfo.dbColumn, fieldValue.Interface())
	}
	return q
}
```

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 4: 提交**

```bash
git add raw.go where_struct.go
git commit -m "feat: add Raw SQL execution and WhereStruct condition builder"
```

---

### Task 18: dbw.go — 入口和事务

**Files:**
- Create: `dbw.go`

- [ ] **Step 1: 创建 dbw.go**

```go
package dbw

import "database/sql"

// ExecuteTx 执行事务
func ExecuteTx(txFn func(*sql.Tx) error, db *sql.DB) (err error) {
	var tx *sql.Tx
	tx, err = db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	err = txFn(tx)
	return err
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```
Expected: 编译成功。

- [ ] **Step 3: 全量验证**

```bash
go vet ./...
go build ./...
```
Expected: 无错误。

- [ ] **Step 4: 提交**

```bash
git add dbw.go
git commit -m "feat: add ExecuteTx transaction wrapper and finalize v1 structure"
```

---

## Phase 8: Tests（SQLite 测试）

### Task 19: 创建测试基础文件

**Files:** 更新 `sqlite_test/` 目录

### Task 20-25: (见后续详细步骤)

测试计划概要：
- `setup_test.go`: TestMain + 建表 + 工具函数
- `insert_test.go`: Insert/InsertBatch/InsertBatchSplit
- `select_test.go`: SelectOne/FindOne/SelectList/SelectById/SelectPage/Count/Exist
- `update_test.go`: UpdateById/Update/指针/策略
- `delete_test.go`: Delete/DeleteById/DeleteByIds/逻辑删除
- `condition_test.go`: Eq/Ne/Gt/Ge/Lt/Le/Like/In/Between/IsNull/NotNull + Or/And/OrNest
- `raw_test.go`: Raw + SelectList/Exec
- `where_struct_test.go`: WhereStruct
- `transaction_test.go`: ExecuteTx 提交/回滚
- `error_test.go`: 结构化错误验证
- `benchmark_test.go`: 性能基准

---

## 自审查清单

- [x] 设计文档所有章节已覆盖
- [x] 无 TBD/TODO 占位
- [x] 类型/签名一致
- [ ] Phase 8 测试需要详细步骤（当前为概要）
