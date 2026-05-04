# DBW v1 重构设计文档

## 1. 概述

对 `github.com/shangjundragon/dbw` 进行完全重写，保持核心设计理念（链式 API + 泛型 + 标签驱动 + Snowflake），同时修复所有已知 Bug、优化性能、重组代码结构、新增 5 项功能。

**核心原则**：轻量 CRUD ORM + 手写 SQL 兜底 + 4 数据库方言 + 单包

**目标数据库**：SQLite、MySQL、PostgreSQL、Oracle

**不纳入 v1**：JOIN、聚合函数、Hooks/Migration、Upsert、复合主键、子查询

## 2. 包结构

所有代码仍在 `dbw` 根包（单 `import`），按职责拆分为以下文件：

```
dbw/
├── dbw.go              # New[T], Options(WithConfig/WithTx/WithContext), ExecuteTx
├── wrapper.go          # DbWrapper[T] 结构体, Reset, Clone, Clean, TableName, Tx, WithContext
├── config.go           # Config, NewConfig, Dialect 接口定义, 驱动常量
├── dialect_mysql.go    # MySQL Dialect
├── dialect_sqlite.go   # SQLite Dialect
├── dialect_postgres.go # PostgreSQL Dialect
├── dialect_oracle.go   # Oracle Dialect
│
├── condition.go        # Where, Eq, Ne, Gt, Ge, Lt, Le, Like, In, Between, IsNull, NotNull
├── condition_group.go  # And, Or, OrNest, 条件性方法: AndIf, WhereIf, EqIf, LikeIf, OrNestIf
│
├── query.go            # Select, OrderBy, OrderByDesc, GroupBy, Having, Distinct, Limit, Offset
├── query_sql.go        # buildSelectSQL, buildWhere, buildUpdateSQL, query, queryRow (内部方法)
├── select.go           # SelectOne, FindOne, SelectList, SelectById, SelectPage, ScanOne, ScanList, ScanPage
├── raw.go              # Raw (原始 SQL 执行+映射)
├── insert.go           # Insert, InsertBatch, InsertBatchSplit, beforeInsert
├── update.go           # UpdateById, Update
├── delete.go           # Delete, DeleteById, DeleteByIds
│
├── meta.go             # structMeta, fieldInfo, getStructMeta, resolveTag, setIdMeta
├── table.go            # getTableName (智能缩写), camelToSnake, Tabler 接口
├── snowflake.go        # Snowflake, GetSnowflake, SetSnowflakeMachineId
├── generator.go        # RegisterIdGenerator, 默认生成器(snowflake/snowflakeStr)
│
├── errors.go           # ErrRecordNotFound, ErrNoWhereClause, ErrNoFieldsToUpdate, ErrTooManyRows 等
├── log.go              # SetLogFn, debugLog
├── convert.go          # convertDefaultValue, getTime
├── utils.go            # GetInt64Ptr, GetStringPtr, GetFloat64Ptr, GetIntPtr, sliceSplit
└── where_struct.go     # WhereStruct (结构体条件查询)
```

## 3. 核心架构变更

### 3.1 Dialect 接口

替代 `switch config.DriverName`，每种数据库一个实现：

```go
type Dialect interface {
    // 数据库驱动名 (sql.Open 的第一个参数)
    DriverName() string

    // 占位符: MySQL→?  PG→$1,$2
    Placeholder(n int) string

    // SQL 转换: 将 ? 替换为数据库原生占位符 (用于原始 SQL)
    ConvertPlaceholders(sql string) string

    // 标识符引用: MySQL→`name`  PG→"name"
    QuoteIdentifier(name string) string

    // 分页 SQL 构建
    BuildPagination(sql string, limit, offset int) string

    // 预编译语句是否支持 (Oracle 有限制)
    SupportsPreparedStatements() bool
}
```

方言选择：`config.DriverName` → 自动匹配 dialect 实现，不匹配则默认 MySQL。

### 3.2 WHERE 条件系统重写

```go
type whereExpr struct {
    sql    string
    args   []any
    joiner string // "AND" 或 "OR"，对于第一个条件则为空
}
```

`Or()` 行为：将**当前最后一条** `whereExpr` 的 `joiner` 设为 `"OR"`，使下一条条件与它用 OR 连接。

### 3.3 预编译语句缓存

`Config` 新增字段 `PreparedStmtCacheSize int`（默认 0 禁用）。内部使用 `sync.Map` 缓存 `*sql.Stmt`，key 为 SQL 模板。

### 3.4 结构化错误

```go
var (
    ErrRecordNotFound     = errors.New("dbw: record not found")
    ErrMultipleRecords    = errors.New("dbw: expected 1 record, got multiple")
    ErrNoWhereClause      = errors.New("dbw: dangerous operation without WHERE clause")
    ErrNoFieldsToUpdate   = errors.New("dbw: no fields to update")
    ErrNoPrimaryKey       = errors.New("dbw: primary key not configured on struct")
    ErrBatchTooLarge      = errors.New("dbw: batch size exceeds maximum limit")
)
```

## 4. Bug 修复（6 项）

| # | 问题 | 重写方案 |
|---|------|---------|
| 1 | `Or()` 失效 | `whereExpr` 添加 `joiner` 字段，`Or()` 修改最后一条的 `joiner="OR"` |
| 2 | `SelectById` 列名错误 | 使用 `q.meta.tableIdDbColumn` 替代 `q.meta.tableIdFiledName` |
| 3 | `Clean()` 丢 config | 保留 `config` 和 `meta` |
| 4 | 主键忽略 `column` 标签 | `setIdMeta()` 检查 `column` 标签优先于 `camelToSnake` |
| 5 | WHERE 逻辑删除污染 | `buildSelectSQL` 内部 clone 后追加逻辑删除条件 |
| 6 | 命名算法不一致 | `camelToSnake` 使用与 `getTableName` 相同的智能缩写算法 |

## 5. 性能优化（6 项）

| # | 优化项 | 重写方案 |
|---|--------|---------|
| 1 | 批量插入 O(n²) | 全部用 `Field(index)` O(1) |
| 2 | 预编译缓存 | `Config.PreparedStmtCacheSize` 控制，无动态参数 SQL 缓存 |
| 3 | Snowflake 竞态 | `atomic.Int64` 保护 `snowflakeMachineId` |
| 4 | `SelectOne` 全量加载 | 追加 `LIMIT 2`，提前终止 |
| 5 | `Count()` 深拷贝 | 临时替换 select 列表，不清除 wheres |
| 6 | SQL 构建分配 | 预计算 `strings.Builder` 容量 |

## 6. 新功能设计（5 项）

### 6.1 Raw SQL 执行+映射
```go
results, err := dbw.New[User](config).
    Raw("SELECT * FROM sys_user WHERE age > ?", 18).
    SelectList()
result, err := dbw.New[User](config).
    Raw("UPDATE sys_user SET age = ? WHERE id = ?", 20, 1).
    Exec()
```

### 6.2 WhereStruct 结构体条件
```go
list, err := dbw.New[User](config).
    WhereStruct(&User{Age: 18, Name: "张三"}).
    SelectList()
```

### 6.3 Limit/Offset 独立
```go
dbw.New[User](config).OrderByDesc("id").Limit(10).Offset(20).SelectList()
```

### 6.4 连接池配置
```go
config := dbw.NewConfig(func(c *dbw.Config) {
    c.Db = db
    c.MaxOpenConns = 10
    c.MaxIdleConns = 5
    c.ConnMaxLifetime = time.Hour
})
```

### 6.5 结构化错误类型
所有错误用 `%w` 包装，支持 `errors.Is`/`errors.As`。

## 7. 公共 API 变更摘要

| 变更 | 详情 |
|------|------|
| 新增 | `Raw(sql, args...)`, `Exec()`, `WhereStruct(data)`, `Limit(n)`, `Offset(n)` |
| 新增 | `Config.MaxOpenConns/MaxIdleConns/ConnMaxLifetime` |
| 新增 | `ErrRecordNotFound` 等错误常量 |
| 重命名 | `PostgreSConverter` → `PostgresConverter` |
| 移除 | `whereCondition` 类型（死代码） |
| 移除 | `whereIsOrIndexes` 字段（被 `joiner` 替代） |
| 行为变更 | `Or()` 正确工作 |
| 行为变更 | `Clean()` 保留 config/meta |
| 行为变更 | `PrintDebugSql` 被内部 `debugLog` 替代 |

## 8. 测试策略

SQLite 自包含测试，每个测试独立数据库、真实断言、table-driven。

```
sqlite_test/
├── setup_test.go        # TestMain
├── insert_test.go
├── select_test.go
├── update_test.go
├── delete_test.go
├── condition_test.go
├── raw_test.go
├── where_struct_test.go
├── transaction_test.go
├── error_test.go
├── benchmark_test.go
└── models_test.go
```
