# DBW - Go 语言数据库封装库

一个简洁、高效的 Go 语言数据库 ORM 封装库，支持 MySQL、SQLite、PostgreSQL 等多种数据库。

## 特性

- ✅ **简洁的 API 设计** - 流畅的链式调用语法
- ✅ **多数据库支持** - MySQL、SQLite、PostgreSQL、Oracle、SQL Server
- ✅ **自动表名映射** - 驼峰命名自动转下划线命名
- ✅ **主键自动生成** - 内置雪花算法生成分布式 ID
- ✅ **逻辑删除** - 支持软删除功能
- ✅ **自动时间戳** - 自动维护创建时间和更新时间
- ✅ **批量操作** - 支持批量插入、分批插入
- ✅ **事务支持** - 完整的事务处理机制
- ✅ **分页查询** - 内置分页功能
- ✅ **调试模式** - 支持 SQL 日志输出
- ✅ **字段级更新策略** - 灵活的更新策略配置

## 安装

```bash
go get github.com/shangjundragon/dbw
```

## 快速开始

### 1. 初始化配置

```go
import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "github.com/shangjundragon/dbw"
)

// 方式一：使用 NewConfig 创建配置
db, err := sql.Open("mysql", "root:password@tcp(localhost:3306)/test?charset=utf8&parseTime=True&loc=Local")
if err != nil {
    log.Fatal(err)
}

config := dbw.NewConfig(func(config *dbw.Config) {
    config.Db = db
    config.Debug = true  // 开启调试模式，打印 SQL
    config.DriverName = "mysql"  // 数据库驱动名
})

// 方式二：使用 InitConfig 设置全局默认配置
dbw.InitConfig(func(config *dbw.Config) {
    config.Db = db
    config.Debug = true
    config.DriverName = "mysql"
})
```

### 2. 定义模型

```go
type User struct {
    Id         int64     `dbw:"primaryKey"`  // 主键，自动生成雪花 ID
    Username   string    `dbw:"default:u"`   // 默认值
    Password   string    
    Age        int       `dbw:"default:0"`   // 默认值
    CreateTime int64     `dbw:"autoCreateTime:milli"`  // 自动创建时间（毫秒）
    UpdateTime int64     `dbw:"autoUpdateTime:milli"`  // 自动更新时间（毫秒）
    DelFlag    string    `dbw:"tableLogic"`  // 逻辑删除标记
}

// 或者实现 Tabler 接口自定义表名
func (User) TableName() string {
    return "sys_user"
}
```

### 3. 基本使用

#### 插入数据

```go
// 单条插入
user := &User{Username: "张三", Age: 18}
result, err := dbw.New[User]().Insert(user)
// user.Id 会自动填充雪花算法生成的 ID

// 批量插入
var users []User
users = append(users, User{Username: "用户 1"}, User{Username: "用户 2"})
result, err := dbw.New[User]().InsertBatch(users)

// 分批批量插入（适合大量数据）
result, err := dbw.New[User]().InsertBatchSplit(users, 100) // 每批 100 条
```

#### 查询数据

```go
// 根据 ID 查询
user, err := dbw.New[User]().SelectById(123)

// 查询单条
user, err := dbw.New[User]().Eq("username", "张三").SelectOne()

// 查询列表
list, err := dbw.New[User]().Gt("age", 18).OrderByDesc("id").SelectList()

// 分页查询
list, count, err := dbw.New[User]().SelectPage(1, 10)

// 选择特定字段
list, err := dbw.New[User]().Select("id", "username", "age").SelectList()

// 计数
count, err := dbw.New[User]().Eq("age", 18).Count()

// 是否存在
exists, err := dbw.New[User]().Eq("username", "张三").Exist()
```

#### 更新数据

```go
// 根据 ID 更新
user := &User{Id: 123, Username: "李四"}
result, err := dbw.New[User]().UpdateById(user)

// 条件更新
result, err := dbw.New[User]().
    Eq("age", 18).
    Update(map[string]any{"username": "王五"})

// 指针类型字段更新策略
type Product struct {
    Id    int64
    Name  string
    Price *float64  // 指针类型，只有非 nil 时才更新
    Stock int       `dbw:"tableUpdateStrategy:always"`  // 总是参与更新
}
```

#### 删除数据

```go
// 根据 ID 删除（支持逻辑删除）
result, err := dbw.New[User]().DeleteById(123)

// 批量删除
result, err := dbw.New[User]().DeleteByIds([]any{1, 2, 3})

// 条件删除
result, err := dbw.New[User]().
    Gt("age", 100).
    Delete()
```

### 4. WHERE 条件

```go
// 等于
dbw.New[User]().Eq("age", 18)

// 不等于
dbw.New[User]().Ne("status", 0)

// 大于/小于
dbw.New[User]().Gt("age", 18)
dbw.New[User]().Lt("age", 60)
dbw.New[User]().Ge("salary", 5000)
dbw.New[User]().Le("price", 100)

// LIKE
dbw.New[User]().Like("username", "%张%")

// IN
dbw.New[User]().In("status", 1, 2, 3)

// BETWEEN
dbw.New[User]().Between("age", 18, 60)

// IS NULL / IS NOT NULL
dbw.New[User]().IsNull("deleted_at")
dbw.New[User]().NotNull("email")

// 条件组合（AND）
dbw.New[User]().
    Eq("age", 18).
    Eq("city", "北京")

// 条件组合（OR）
dbw.New[User]().
    Eq("age", 18).
    Or().
    Eq("vip_level", 1)

// 嵌套条件
dbw.New[User]().
    Eq("status", 1).
    And(func(w *dbw.DbWrapper[User]) {
        w.Eq("age", 18).Or().Eq("vip_level", 1)
    })

// 条件判断
age := 18
dbw.New[User]().
    WhereIf(age > 0, "age > ?", age).
    EqIf(city != "", "city", city)
```

### 5. 排序、分组和聚合

```go
// 排序
dbw.New[User]().OrderByDesc("create_time").OrderBy("id")

// 分组
dbw.New[User]().
    Select("city", "COUNT(*) as count").
    GroupBy("city")

// HAVING
dbw.New[User]().
    Select("city", "AVG(age) as avg_age").
    GroupBy("city").
    Having("AVG(age) > ?", 30)

// DISTINCT
dbw.New[User]().Distinct().Select("city")
```

### 6. 事务处理

```go
// 使用 ExecuteTx 执行事务
err := dbw.ExecuteTx(func(tx *sql.Tx) error {
    // 在事务中操作
    _, err := dbw.New[User](dbw.WithTx(tx)).Insert(&User{Username: "用户 1"})
    if err != nil {
        return err
    }
    
    _, err = dbw.New[Order](dbw.WithTx(tx)).Insert(&Order{UserId: 1})
    if err != nil {
        return err
    }
    
    return nil
}, config.Db)

if err != nil {
    // 事务回滚
    log.Fatal(err)
}
// 事务提交
```

### 7. 高级功能

#### 自定义 ID 生成器

```go
// 注册自定义 ID 生成器
dbw.RegisterIdGenerator("custom", func() any {
    return generateCustomId()
})

// 在模型中使用
type Product struct {
    Id int64 `dbw:"primaryKey;idGenerator:custom"`
}
```

#### 占位符转换

```go
// PostgreSQL 使用 $1, $2 占位符
config.PlaceholderConverter = dbw.PostgreSConverter

// MySQL 使用 ? 占位符（默认）
config.PlaceholderConverter = dbw.MySQLConverter
```

#### 分页拦截器

```go
// 自定义分页逻辑
config.PageInterceptor = func(sqlStr string, pageNum int, pageSize int) string {
    // 自定义分页 SQL
    offset := (pageNum - 1) * pageSize
    return fmt.Sprintf("%s LIMIT %d OFFSET %d", sqlStr, pageSize, offset)
}
```

#### 上下文支持

```go
// 设置上下文
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

user, err := dbw.New[User]().
    WithContext(ctx).
    SelectById(123)
```

## 标签说明

| 标签 | 说明 | 示例 |
|------|------|------|
| `primaryKey` | 主键标识 | `dbw:"primaryKey"` |
| `column` | 指定列名 | `dbw:"column:user_name"` |
| `dbIgnore` | 忽略该字段 | `dbw:"dbIgnore:true"` |
| `default` | 默认值 | `dbw:"default:0"` |
| `idGenerator` | ID 生成器 | `dbw:"idGenerator:snowflake"` |
| `tableLogic` | 逻辑删除 | `dbw:"tableLogic"` |
| `autoCreateTime` | 自动创建时间 | `dbw:"autoCreateTime:milli"` |
| `autoUpdateTime` | 自动更新时间 | `dbw:"autoUpdateTime"` |
| `tableUpdateStrategy` | 更新策略 | `dbw:"tableUpdateStrategy:always"` |

## 支持的数据库

- ✅ MySQL
- ✅ SQLite
- ✅ PostgreSQL
- ✅ Oracle
- ✅ SQL Server

## 注意事项

1. **主键要求**：批量插入时，如果使用自动 ID 生成器，主键必须唯一
2. **更新安全**：不带 WHERE 条件的更新会被阻止（除非使用 `UpdateById`）
3. **删除安全**：不支持不带 WHERE 条件的删除操作
4. **逻辑删除**：启用逻辑删除后，查询会自动添加 `del_flag = '0'` 条件
5. **时间格式**：支持秒级（time.Time）和毫秒级（int64）时间戳

## 测试

```bash
# 运行 MySQL 测试
go test ./mysql_test

# 运行 SQLite 测试
go test ./sqlite_test
```

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！

## 联系方式

如有问题或建议，请通过以下方式联系：
- GitHub Issues: https://github.com/shangjundragon/dbw/issues
