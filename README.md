# DBW — Go 语言轻量级 ORM 库

一个类型安全、高性能的 Go 语言 ORM 库，支持 MySQL、SQLite、PostgreSQL、Oracle 四种数据库。基于泛型实现编译时类型检查，提供流畅的链式 API。

## 特性

- **类型安全** — Go 1.21+ 泛型，编译时类型检查，无需 `interface{}` 断言
- **链式 API** — 流畅的 `dbw.New[User](config).Eq("age", 18).OrderByDesc("id").SelectList()`
- **四数据库支持** — MySQL、SQLite、PostgreSQL、Oracle，Dialect 接口自动适配占位符和分页语法
- **Snowflake ID** — 内置雪花算法生成分布式主键，支持自定义 ID 生成器
- **逻辑删除** — 自动在查询/更新/删除时追加逻辑删除条件
- **自动时间戳** — `autoCreateTime` / `autoUpdateTime` 自动维护，支持秒级和毫秒级
- **安全防护** — `Update()` 和 `Delete()` 无 WHERE 条件时报错
- **批量操作** — `InsertBatch`（上限 1000）、`InsertBatchSplit` 自动分批
- **事务支持** — `ExecuteTx` 事务包装器
- **结构化错误** — `ErrRecordNotFound` / `ErrNoWhereClause` 等，支持 `errors.Is()` 判断
- **条件性构建** — `WhereIf` / `AndIf` / `EqIf` / `LikeIf` 避免 `if-else` 嵌套
- **原始 SQL** — `Raw()` + `SelectList()` / `Exec()` 执行任意 SQL
- **结构体条件** — `WhereStruct(&user)` 非零字段自动转 WHERE
- **生命周期钩子** — 全局/实例级 BeforeInsert、AfterInsert、BeforeUpdate、AfterUpdate、BeforeDelete、AfterDelete、AfterQuery
- **零依赖** — 核心库无第三方依赖（Snowflake 自实现）

## 安装

```bash
go get github.com/shangjundragon/dbw
```

按需引入驱动：

```go
import (
    _ "github.com/go-sql-driver/mysql"      // MySQL
    _ "github.com/glebarez/go-sqlite"        // SQLite (CGO-free)
    _ "github.com/lib/pq"                    // PostgreSQL
    _ "github.com/sijms/go-ora/v2"           // Oracle
)
```

## 快速开始

### 1. 初始化配置

```go
package main

import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "github.com/shangjundragon/dbw"
)

func main() {
    db, _ := sql.Open("mysql", "root:password@tcp(localhost:3306)/test?charset=utf8&parseTime=True&loc=Local")

    // 通过回调函数创建配置
    config := dbw.NewConfig(func(c *dbw.Config) {
        c.Db = db
        c.DriverName = "mysql"   // mysql | sqlite | postgres | oracle
        c.Debug = true           // 打印 SQL 日志
        c.MaxOpenConns = 10      // 连接池配置
        c.MaxIdleConns = 5
    })
}
```

### 2. 定义模型

```go
type User struct {
    Id         int64     `dbw:"primaryKey"`            // 主键，自动生成 Snowflake ID
    Username   string    `dbw:"default:u"`             // 插入时零值默认值
    Password   string
    NickName   *string   `dbw:"column:nick_name"`      // 指针类型，更新时 nil=跳过
    Age        int       `dbw:"default:0"`
    CreateTime int64     `dbw:"autoCreateTime:milli"`  // 创建时自动填充毫秒时间戳
    UpdateTime int64     `dbw:"autoUpdateTime:milli"`  // 创建和更新时自动填充
    DelFlag    string    `dbw:"tableLogic"`            // 逻辑删除标记
}

// 可选：自定义表名
func (User) TableName() string {
    return "sys_user"
}
```

### 3. 基本 CRUD

```go
// === 插入 ===
user := &User{Username: "张三", Age: 18}
result, _ := dbw.New[User](dbw.WithConfig(config)).Insert(user)
// user.Id 已被自动填充

// 批量插入（上限 1000 条）
users := []User{{Username: "a"}, {Username: "b"}}
dbw.New[User](dbw.WithConfig(config)).InsertBatch(users)

// 超大批量插入（自动分批）
dbw.New[User](dbw.WithConfig(config)).InsertBatchSplit(users, 500)

// === 查询 ===
// 按 ID 查（0 或 1 条，多条报错）
user, _ := dbw.New[User](dbw.WithConfig(config)).SelectById(123)

// 按 ID 列表查
users, _ := dbw.New[User](dbw.WithConfig(config)).SelectByIds([]any{1, 2, 3})

// 查单条（多条返回第一条，无结果报错 ErrRecordNotFound）
user, _ := dbw.New[User](dbw.WithConfig(config)).Eq("username", "张三").FindOne()

// 查单条（多条报错 ErrMultipleRecords，0 条返回 nil）
user, _ := dbw.New[User](dbw.WithConfig(config)).Eq("username", "张三").SelectOne()

// 查列表
list, _ := dbw.New[User](dbw.WithConfig(config)).Gt("age", 18).SelectList()

// 分页查询
list, total, _ := dbw.New[User](dbw.WithConfig(config)).SelectPage(1, 10)

// 计数 / 存在性
count, _ := dbw.New[User](dbw.WithConfig(config)).Eq("age", 18).Count()
exists, _ := dbw.New[User](dbw.WithConfig(config)).Eq("id", 1).Exist()

// === 更新 ===
// 按结构体更新（主键自动生成 WHERE 条件）
user := &User{Id: 123, Age: 31}
dbw.New[User](dbw.WithConfig(config)).UpdateById(user)

// 按 map 更新（必须带 WHERE 条件）
dbw.New[User](dbw.WithConfig(config)).
    Eq("age", 18).
    Update(map[string]any{"username": "李四"})

// === 删除 ===
// 逻辑删除（自动执行 UPDATE del_flag = '1'）
dbw.New[User](dbw.WithConfig(config)).DeleteById(123)
dbw.New[User](dbw.WithConfig(config)).DeleteByIds([]any{1, 2, 3})

// 带条件删除
dbw.New[User](dbw.WithConfig(config)).Gt("age", 100).Delete()
```

## WHERE 条件

### 基础条件

```go
dbw.New[User](config).
    Eq("age", 18).          // age = ?
    Ne("status", 0).        // status != ?
    Gt("age", 18).          // age > ?
    Ge("age", 18).          // age >= ?
    Lt("age", 60).          // age < ?
    Le("age", 60).          // age <= ?
    Like("username", "%张%").// username LIKE ?
    In("status", 1, 2, 3).  // status IN (?,?,?)
    Between("age", 18, 60). // age BETWEEN ? AND ?
    IsNull("deleted_at").   // deleted_at IS NULL
    NotNull("email").       // email IS NOT NULL
    SelectList()
```

### OR 条件

```go
// 简单 OR（标记下一条条件用 OR 连接）
dbw.New[User](config).
    Eq("age", 18).
    Or().
    Eq("vip_level", 1).
    SelectList()
// 生成: WHERE age = ? OR vip_level = ?

// OR 嵌套
dbw.New[User](config).
    Eq("status", 1).
    OrNest(func(w *dbw.DbWrapper[User]) {
        w.Eq("age", 18).Or().Eq("vip_level", 1)
    }).
    SelectList()
// 生成: WHERE status = ? OR (age = ? OR vip_level = ?)
```

### AND 嵌套

```go
dbw.New[User](config).
    Eq("city", "北京").
    And(func(w *dbw.DbWrapper[User]) {
        w.Eq("age", 18).Or().Eq("vip_level", 1)
    }).
    SelectList()
// 生成: WHERE city = ? AND (age = ? OR vip_level = ?)
```

### 条件性构建

```go
age := 18
city := ""

dbw.New[User](config).
    WhereIf(age > 0, "age > ?", age).    // 条件为 true 时生效
    EqIf(city != "", "city", city).       // 条件为 false 时跳过
    AndIf(len(ids) > 0, func(w *dbw.DbWrapper[User]) {
        w.In("id", ids...)
    }).
    SelectList()
```

## 查询构建

### 选择字段 & 排序 & 分组

```go
dbw.New[User](config).
    Select("id", "username", "age").   // 指定字段
    OrderByDesc("create_time").        // 降序
    OrderBy("id").                     // 升序
    GroupBy("city").                   // 分组
    Having("COUNT(*) > ?", 5).         // HAVING 条件
    Distinct().                        // 去重
    SelectList()
```

### Limit / Offset

```go
// 不依赖分页也能用
dbw.New[User](config).
    OrderByDesc("id").
    Limit(10).Offset(20).
    SelectList()
```

### 分页

```go
// SelectPage 默认 pageSize=10
list, total, _ := dbw.New[User](config).
    OrderByDesc("id").
    SelectPage(1, 10)

// 自定义分页拦截器
config.PageInterceptor = func(sql string, pageNum, pageSize int) string {
    return fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, pageSize, (pageNum-1)*pageSize)
}
```

### 原始扫描

```go
// ScanOne / ScanList / ScanPage — 手动处理 *sql.Rows
dbw.New[User](config).ScanList(func(rows *sql.Rows) error {
    var id int64
    var name string
    rows.Scan(&id, &name)
    // ...
    return nil
})
```

## 查询方法对比

| 方法 | 0 条结果 | 多条结果 | 用途 |
|------|---------|---------|------|
| `SelectById` | `nil, nil` | `ErrMultipleRecords` | 按主键精确查 |
| `SelectByIds` | 空切片 `[]T{}` | — | 按主键批量查 |
| `SelectOne` | `nil, nil` | `ErrMultipleRecords` | 严格只能匹配一条 |
| `FindOne` | `ErrRecordNotFound` | 返回第一条 | 宽松取一条 |
| `SelectList` | 空切片 `[]T{}` | 返回全部 | 列表查询 |
| `SelectPage` | 空切片 `[]T{}`, `count=0` | 返回分页结果 | 分页查询 |

## 事务

```go
err := dbw.ExecuteTx(func(tx *sql.Tx) error {
    _, err := dbw.New[User](dbw.WithConfig(config), dbw.WithTx(tx)).
        Insert(&User{Username: "新用户"})
    if err != nil {
        return err
    }
    _, err = dbw.New[Order](dbw.WithConfig(config), dbw.WithTx(tx)).
        Insert(&Order{UserId: 1})
    return err
}, config.Db)

if err != nil {
    log.Println("事务回滚:", err)
}
```

## 原始 SQL

```go
// 查询 — 自动映射到结构体
users, _ := dbw.New[User](config).
    Raw("SELECT * FROM sys_user WHERE age > ? AND status = ?", 18, 1).
    SelectList()

// 写操作
result, _ := dbw.New[User](config).
    Raw("UPDATE sys_user SET age = ? WHERE id = ?", 20, 1).
    Exec()
```

## WhereStruct — 结构体条件

```go
// 非零字段自动转为 Eq 条件，零值和 nil 指针跳过
user := &User{Age: 18, Username: "张三"}
list, _ := dbw.New[User](config).
    WhereStruct(user).
    SelectList()
// 生成: WHERE age = ? AND username = ?
```

## 结构化错误

```go
import "errors"

user, err := dbw.New[User](config).SelectById(999)
if errors.Is(err, dbw.ErrRecordNotFound) {
    // 处理未找到
}
if errors.Is(err, dbw.ErrMultipleRecords) {
    // 处理多条记录
}
```

| 错误常量 | 触发场景 |
|---------|---------|
| `ErrRecordNotFound` | `FindOne` 无结果 |
| `ErrMultipleRecords` | `SelectOne` / `SelectById` 多行匹配 |
| `ErrNoWhereClause` | `Update()` / `Delete()` 无 WHERE |
| `ErrNoFieldsToUpdate` | `Insert` 或 `UpdateById` 无有效字段 |
| `ErrNoPrimaryKey` | 结构体缺少主键标识 |
| `ErrBatchTooLarge` | `InsertBatch` 超 1000 条 |
| `ErrEmptyData` | 传入空切片 |
| `ErrNilEntity` | 传入 nil 指针 |

## 数据库方言

根据 `config.DriverName` 自动选择方言，也可手动指定：

```go
config := dbw.NewConfig(func(c *dbw.Config) {
    c.Db = db
    c.DriverName = "postgres" // 自动选择 PostgreSQL 方言
    // 或手动指定:
    // c.Dialect = &dbw.PostgresDialect{}
})

// 自定义方言
type MyDialect struct{}
func (d *MyDialect) DriverName() string { return "my-db" }
func (d *MyDialect) Placeholder(n int) string { return "?" }
func (d *MyDialect) ConvertPlaceholders(sql string) string { return sql }
func (d *MyDialect) QuoteIdentifier(name string) string { return "[" + name + "]" }
func (d *MyDialect) BuildPagination(sql string, limit, offset int) string {
    return fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, limit, offset)
}
```

方言差异自动处理：

| 数据库 | 占位符 | 分页语法 | 标识符引号 |
|--------|--------|---------|-----------|
| MySQL | `?` | `LIMIT ? OFFSET ?` | `` `name` `` |
| SQLite | `?` | `LIMIT ? OFFSET ?` | `"name"` |
| PostgreSQL | `$1, $2` | `LIMIT ? OFFSET ?` | `"name"` |
| Oracle | `:1, :2` | `OFFSET ? ROWS FETCH NEXT ? ROWS ONLY` | `"name"` |

## ID 生成器

### 内置生成器

```go
// snowflake（默认） — int/int64/uint64 类型主键
type User struct {
    Id int64 `dbw:"primaryKey"` // 自动 snowflake
}

// snowflakeStr — string 类型主键
type Order struct {
    Id string `dbw:"primaryKey"` // 自动 snowflakeStr
}
```

### 自定义生成器

```go
import "github.com/google/uuid"

// 注册
dbw.RegisterIdGenerator("uuid", func() any {
    return uuid.New().String()
})

// 使用
type File struct {
    Id string `dbw:"primaryKey;idGenerator:uuid"`
}
```

### Snowflake 配置

```go
dbw.SetSnowflakeMachineId(5) // 设置机器 ID（默认 1），需在首次 GetSnowflake 前调用
```

## 生命周期钩子

提供在执行 Insert、Update、Delete、Select 操作时的回调点，支持**类型泛型钩子**和**通用实体钩子**两种机制。

### 类型泛型钩子（Hooks[T]）

绑定到特定实体类型，支持**全局注册**和**实例级注入**，常用于 AOP 拦截。

#### Hook 类型

| 钩子 | 触发时机 | 签名 |
|------|---------|------|
| `BeforeInsert` | Insert 执行前 | `func(q *DbWrapper[T], data *T) error` |
| `AfterInsert` | Insert 成功后 | `func(q *DbWrapper[T], data *T, result sql.Result) error` |
| `BeforeUpdate` | UpdateById 执行前 | `func(q *DbWrapper[T], data *T) error` |
| `BeforeUpdateMap` | Update(map) 执行前 | `func(q *DbWrapper[T], values map[string]any) error` |
| `AfterUpdate` | Update/UpdateById 成功后 | `func(q *DbWrapper[T], result sql.Result) error` |
| `BeforeDelete` | Delete 执行前 | `func(q *DbWrapper[T]) error` |
| `AfterDelete` | Delete 成功后 | `func(q *DbWrapper[T], result sql.Result) error` |
| `AfterQuery` | Select 每行扫描后 | `func(q *DbWrapper[T], data *T) error` |

所有钩子均为可选（nil 跳过），返回 error 时中断当前操作。

#### 全局注册（RegisterHooks）

进程级生效，适合设置全局基线行为：

```go
dbw.RegisterHooks[User](func(h *dbw.Hooks[User]) {
    h.BeforeInsert = func(q *dbw.DbWrapper[User], data *User) error {
        data.NickName = getCurrentUser(q.Ctx())
        return nil
    }
    h.AfterQuery = func(q *dbw.DbWrapper[User], data *User) error {
        data.Password = "***" // 返回时脱敏
        return nil
    }
})
```

多次调用 `RegisterHooks[T]` 会合并到同一个 `Hooks[T]`，适合在不同模块中分步注册。

#### 实例级注入（WithHooks）

只对当前 `DbWrapper` 实例生效，优先级高于全局 `Hooks[T]`：

```go
dbw.New[User](dbw.WithConfig(config), dbw.WithHooks(func(h *dbw.Hooks[User]) {
    h.BeforeInsert = func(q *dbw.DbWrapper[User], data *User) error {
        data.Username = strings.ToLower(data.Username)
        return nil
    }
}))
```

### 通用实体钩子（EntityHook）

不绑定具体类型，对所有实体类型生效。通过反射检查字段或 tag，适合通用的自动填充逻辑（如从 Context 中获取用户 ID 写入 `create_by` 字段）：

```go
dbw.RegisterEntityHook(func(ctx context.Context, point dbw.HookPoint, entity any) error {
    if point != dbw.HookBeforeInsert && point != dbw.HookBeforeUpdate {
        return nil
    }
    v := reflect.ValueOf(entity).Elem()
    t := v.Type()
    for i := 0; i < t.NumField(); i++ {
        tagMap := dbw.ResolveDbwTag(t.Field(i).Tag.Get("dbw"))
        if tagMap["autoCreateUser"] == "true" || tagMap["autoUpdateUser"] == "true" {
            v.Field(i).Set(reflect.ValueOf(ctx.Value("user_id")))
        }
    }
    return nil
})
```

配合 tag 使用：

```go
type Entity struct {
    Id       int64  `dbw:"primaryKey"`
    Name     string
    CreateBy int64  `dbw:"autoCreateUser"`
    UpdateBy int64  `dbw:"autoUpdateUser"`
}
```

`RegisterEntityHook` 支持多次调用，所有注册的钩子按注册顺序执行。

### 执行顺序（完整链路）

```
EntityHook → 全局 Hooks[T] → 实例 Hooks[T]
```

以 Insert 为例：

```
1. beforeInsert (自动填充主键/时间戳)
2. EntityHook(HookBeforeInsert, data)  ← 通用
3. Hooks[T] BeforeInsert (全局)         ← 类型特定
4. Hooks[T] BeforeInsert (实例)         ← 实例覆盖
5. 构建 SQL + 执行
6. Hooks[T] AfterInsert (实例)
7. Hooks[T] AfterInsert (全局)
8. EntityHook(HookAfterInsert, data)
```

### 错误传播

任意钩子返回 error 会中断操作：
- Before 钩子返回 error → SQL 不执行
- After 钩子返回 error → SQL 已执行（无法回滚，除非在事务中）

```go
dbw.RegisterEntityHook(func(ctx context.Context, point dbw.HookPoint, entity any) error {
    v := reflect.ValueOf(entity).Elem()
    if v.FieldByName("Username").String() == "" {
        return fmt.Errorf("username is required")
    }
    return nil
})
```

## 标签参考

标签分隔符为 `;`，如 `dbw:"primaryKey;idGenerator:uuid"`。

| 标签 | 值 | 说明 |
|------|-----|------|
| `primaryKey` | — | 主键字段，自动生成 ID |
| `idGenerator` | `snowflake` / `snowflakeStr` / 自定义 | 指定 ID 生成器 |
| `tableLogic` | — | 逻辑删除标记（查询/更新/删除自动追加未删除条件） |
| `autoCreateTime` | 空 / `milli` | 插入时自动填充创建时间（空=time.Time, milli=int64毫秒） |
| `autoUpdateTime` | 空 / `milli` | 插入和更新时自动填充更新时间 |
| `default` | 任意值 | 插入时字段零值的默认值 |
| `dbIgnore` | `true` | 忽略该字段（不映射到数据库） |
| `column` | 列名 | 覆盖数据库列名 |
| `tableUpdateStrategy` | `always` | 更新时零值/nil 也参与 SET |

### 更新策略详解

```go
type Product struct {
    Id    int64    `dbw:"primaryKey"`
    Name  string
    Price *float64                            // 指针 nil → 不更新
    Stock int      `dbw:"tableUpdateStrategy:always"` // 零值也更新
}

// Price 为 nil → 只更新 Name 和 Stock
// Price 非 nil → 同时更新 Price
// Stock 始终参与更新（即使为 0）
dbw.New[Product](config).UpdateById(&Product{Id: 1, Name: "新名称", Stock: 0})
```

## 表名规则

默认将 Go 类型名从 PascalCase 转换为 snake_case，支持智能缩写处理：

| 结构体名 | 表名 |
|---------|------|
| `User` | `user` |
| `UserInfo` | `user_info` |
| `HTTPServer` | `http_server` |
| `OAuthClient` | `oauth_client` |

```go
// 方式一：实现 Tabler 接口（编译时）
func (User) TableName() string { return "sys_user" }

// 方式二：实例级覆盖（运行时）
dbw.New[User](config).TableName("sys_user_backup").SelectList()
```

## 日志

```go
// 内置日志
config.Debug = true // 打印 SQL + 参数

// 自定义日志（集成到自己的日志系统）
dbw.SetLogFn(func(sqlStr string, args []any, ctx context.Context) {
    log.Printf("[DBW] SQL: %s, Args: %v", sqlStr, args)
})
```

## 上下文

```go
ctx, cancel := dbw.GetContextWithTimeout(5 * time.Second)
defer cancel()

user, _ := dbw.New[User](dbw.WithConfig(config), dbw.WithContext(ctx)).
    SelectById(123)
```

## 连接池

```go
config := dbw.NewConfig(func(c *dbw.Config) {
    c.Db = db
    c.MaxOpenConns = 25
    c.MaxIdleConns = 10
    c.ConnMaxLifetime = time.Hour
})
```

## 测试

```bash
# SQLite 测试（自包含，无需外部数据库）
go test ./sqlite_test

# SQLite 性能基准
go test ./sqlite_test -bench=.

# MySQL 测试（需修改 DSN）
go test ./mysql_test

# 静态检查
go vet ./...
```

## 项目结构

```
dbw/
├── wrapper.go          # DbWrapper[T] 核心结构体 + New/Options/ExecuteTx
├── config.go           # Config, Dialect 接口, NewConfig
├── dialect_*.go        # MySQL/SQLite/PostgreSQL/Oracle 方言实现
├── condition.go        # Eq/Ne/Gt/Lt/In/Between/IsNull/NotNull 等
├── condition_group.go  # Or/And/OrNest/WhereIf 等条件分组
├── query.go            # Select/OrderBy/GroupBy/Limit/Offset/Count
├── query_sql.go        # SQL 生成（内部）
├── select.go           # SelectOne/FindOne/SelectList/SelectPage
├── insert.go           # Insert/InsertBatch/InsertBatchSplit
├── update.go           # UpdateById/Update
├── delete.go           # Delete/DeleteById/DeleteByIds
├── raw.go              # Raw + Exec 原始 SQL
├── where_struct.go     # WhereStruct 结构体条件
├── meta.go             # 结构体元数据 + 标签解析
├── table.go            # 表名生成 + Tabler 接口
├── snowflake.go        # Snowflake ID 生成器
├── hooks.go            # 生命周期钩子（Hooks[T], RegisterHooks, WithHooks, EntityHook, RegisterEntityHook）
├── errors.go           # 结构化错误类型
├── log.go              # 日志系统
├── convert.go          # 默认值转换 + 时间处理
├── utils.go            # 辅助函数
├── sqlite_test/        # SQLite 自包含测试
└── mysql_test/         # MySQL 测试
```

## 许可证

MIT License
