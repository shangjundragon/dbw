# dbw - Go语言数据库包装器

一个基于泛型的轻量级数据库操作包装器，提供了简洁的链式API来执行常见的数据库操作。

## 特性

- **泛型支持**：使用Go 1.21+的泛型特性，提供类型安全的操作
- **链式调用**：流畅的API设计，支持链式调用
- **自动映射**：自动将数据库记录映射到Go结构体
- **事务支持**：支持事务操作
- **逻辑删除**：内置逻辑删除支持
- **自动时间戳**：支持自动填充创建时间和更新时间
- **批量操作**：支持批量插入和分批插入
- **分页查询**：内置分页功能
- **多数据库支持**：支持MySQL、SQLite等主流数据库

## 安装

```bash
go get github.com/shangjundragon/dbw
```

## 快速开始

### 1. 初始化配置

```go
package main

import (
    "database/sql"
    "log"
    "github.com/shangjundragon/dbw"
    _ "github.com/go-sql-driver/mysql" // MySQL驱动
    // 或者 _ "github.com/glebarez/go-sqlite" // SQLite驱动
)

func main() {
    // 连接数据库
    db, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local")
    if err != nil {
        log.Fatal(err)
    }

    // 初始化dbwrapper配置
    dbw.InitConfig(func(config *dbw.Config) {
        config.Db = db
        config.Debug = true // 开启调试模式，打印SQL语句
        config.DriverName = "mysql" // 设置数据库驱动
    })

    // 现在可以使用dbwrapper了
}
```

### 2. 定义模型结构

```go
type User struct {
    Id         int64     `dbw:"primaryKey"` // 主键标识
    Username   string    `db:"username"`
    Email      string    `db:"email"`
    Age        int       `dbw:"default:18"` // 默认值
    Status     int       `db:"status"`
    CreateTime time.Time `dbw:"autoCreateTime"` // 自动创建时间
    UpdateTime time.Time `dbw:"autoUpdateTime"` // 自动更新时间
    DelFlag    string    `dbw:"tableLogic"`     // 逻辑删除标志
}
```

### 3. 基本CRUD操作

#### 插入数据

```go
// 单条插入
user := &User{
    Username: "john_doe",
    Email:    "john@example.com",
    Age:      25,
    Status:   1,
}

affected, err := dbw.NewQuery[User]().Insert(user)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("插入成功，影响行数: %d\n", affected)
fmt.Printf("插入后的ID: %d\n", user.Id)
```

#### 批量插入

```go
// 批量插入
users := []User{
    {Username: "user1", Email: "user1@example.com", Age: 20},
    {Username: "user2", Email: "user2@example.com", Age: 25},
    {Username: "user3", Email: "user3@example.com", Age: 30},
}

affected, err := dbw.NewQuery[User]().InsertBatch(users)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("批量插入成功，影响行数: %d\n", affected)
```

#### 查询操作

```go
// 查询单条记录
user, err := dbw.NewQuery[User]().Eq("id", 1).SelectOne()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("用户: %+v\n", user)

// 根据ID查询
user, err = dbw.NewQuery[User]().SelectById(1)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("用户: %+v\n", user)

// 查询多条记录
users, err := dbw.NewQuery[User]().Eq("status", 1).SelectList()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("用户列表长度: %d\n", len(users))

// 条件查询
users, err = dbw.NewQuery[User]().
    Like("username", "%john%").
    Gt("age", 18).
    OrderByDesc("create_time").
    SelectList()
if err != nil {
    log.Fatal(err)
}

// 分页查询
records, count, err := dbw.NewQuery[User]().SelectPage(1, 10)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("总记录数: %d, 当前页记录数: %d\n", count, len(records))

// 指定查询字段
users, err = dbw.NewQuery[User]().Select("id", "username", "email").SelectList()
if err != nil {
    log.Fatal(err)
}
```

#### 更新操作

```go
// 根据ID更新
user := &User{
    Id:       1,
    Username: "updated_username",
    Email:    "updated@example.com",
    Age:      30,
}

affected, err := dbw.NewQuery[User]().UpdateById(user)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("更新成功，影响行数: %d\n", affected)

// 根据条件更新
affected, err = dbw.NewQuery[User]().
    Eq("status", 1).
    Update(map[string]any{
        "status": 0,
        "email":  "inactive@example.com",
    })
if err != nil {
    log.Fatal(err)
}
fmt.Printf("条件更新成功，影响行数: %d\n", affected)
```

#### 删除操作

```go
// 根据ID删除（如果是逻辑删除，会更新删除标志）
affected, err := dbw.NewQuery[User]().DeleteById(1)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("删除成功，影响行数: %d\n", affected)

// 条件删除
affected, err = dbw.NewQuery[User]().
    Eq("status", 0).
    Delete()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("条件删除成功，影响行数: %d\n", affected)
```

### 4. 高级查询功能

#### 条件查询

```go
// 等于
users, _ := dbw.NewQuery[User]().Eq("username", "john").SelectList()

// 不等于
users, _ := dbw.NewQuery[User]().Ne("status", 0).SelectList()

// 大于/小于
users, _ := dbw.NewQuery[User]().Gt("age", 18).Lt("age", 65).SelectList()

// IN查询
users, _ := dbw.NewQuery[User]().In("id", 1, 2, 3, 4, 5).SelectList()

// LIKE查询
users, _ := dbw.NewQuery[User]().Like("username", "%admin%").SelectList()

// BETWEEN查询
users, _ := dbw.NewQuery[User]().Between("age", 18, 65).SelectList()

// NULL/NOT NULL
users, _ := dbw.NewQuery[User]().IsNull("email").SelectList()
users, _ := dbw.NewQuery[User]().NotNull("email").SelectList()

// AND/OR组合条件
users, _ := dbw.NewQuery[User]().
    Eq("status", 1).
    And(func(q *dbw.DbWrapper[User]) {
        q.Gt("age", 18).Lt("age", 65)
    }).
    OrNest(func(q *dbw.DbWrapper[User]) {
        q.Like("username", "%admin%").Eq("role", "admin")
    }).
    SelectList()
```

#### 排序、分组和聚合

```go
// 排序
users, _ := dbw.NewQuery[User]().
    OrderBy("username").
    OrderByDesc("create_time").
    SelectList()

// 分组查询
count, err := dbw.NewQuery[User]().GroupBy("status").Count()

// 去重查询
users, _ := dbw.NewQuery[User]().Distinct().Select("status").SelectList()
```

### 5. 事务操作

```go
err := dbw.ExecuteTx(func(tx *sql.Tx) error {
    // 在事务中执行多个操作
    user1 := &User{Username: "user1", Email: "user1@example.com"}
    _, err := dbw.NewQuery[User]().Tx(tx).Insert(user1)
    if err != nil {
        return err // 事务回滚
    }

    user2 := &User{Username: "user2", Email: "user2@example.com"}
    _, err = dbw.NewQuery[User]().Tx(tx).Insert(user2)
    if err != nil {
        return err // 事务回滚
    }

    return nil // 提交事务
})

if err != nil {
    log.Printf("事务执行失败: %v", err)
} else {
    log.Println("事务执行成功")
}
```

### 6. 结构体标签说明

| 标签 | 说明               |
|------|------------------|
| `dbw:"primaryKey"` | 标识主键字段           |
| `dbw:"tableLogic"` | 标识逻辑删除字段         |
| `dbw:"autoCreateTime"` | 自动填充创建时间（秒级时间戳）  |
| `dbw:"autoCreateTime:milli"` | 自动填充创建时间（毫秒级时间戳） |
| `dbw:"autoUpdateTime"` | 自动填充更新时间（秒级时间戳）  |
| `dbw:"autoUpdateTime:milli"` | 自动填充更新时间（毫秒级时间戳） |
| `dbw:"default:value"` | 设置默认值            |
| `dbw:"idType:auto"` | 标识自增主键           |
| `dbw:"idType:assign"` | 标识自动分配ID的主键      |
| `dbw:"tableUpdateStrategy:always"` | 标识该字段总是参与更新操作    |

### 7. 配置选项

```go
dbw.InitConfig(func(config *dbw.Config) {
    config.Db = db                              // 数据库连接
    config.Debug = true                         // 调试模式，打印SQL语句
    config.DriverName = "mysql"                 // 数据库驱动名称
    config.LogicDeleteValue = "1"              // 逻辑删除值
    config.LogicNotDeleteValue = "0"           // 逻辑未删除值
    config.GetSnowFlakeMachineId = func() int64 { return 1 } // 雪花算法机器ID
    config.GenerateTableId = func() any { return dbw.GetSnowflake().GetId() } // ID生成器
    
    // 自定义分页拦截器
    config.PageInterceptor = func(sqlStr string, pageNum int, pageSize int) (finalSql string) {
        offset := (pageNum - 1) * pageSize
        return sqlStr + fmt.Sprintf(" LIMIT %d OFFSET %d", pageSize, offset)
    }
})
```

## 支持的方法

### 查询方法
- `Select()` - 指定查询字段
- `Eq()` - 等于
- `Ne()` - 不等于
- `Gt()` - 大于
- `Ge()` - 大于等于
- `Lt()` - 小于
- `Le()` - 小于等于
- `Like()` - LIKE查询
- `In()` - IN查询
- `Between()` - BETWEEN查询
- `IsNull()` - IS NULL
- `NotNull()` - IS NOT NULL
- `Where()` - 自定义WHERE条件
- `OrderBy()` - 升序排序
- `OrderByDesc()` - 降序排序
- `GroupBy()` - 分组
- `Having()` - HAVING条件
- `Distinct()` - 去重
- `SelectOne()` - 查询单条记录
- `SelectList()` - 查询多条记录
- `SelectById()` - 根据ID查询
- `SelectPage()` - 分页查询
- `Count()` - 统计数量
- `Exist()` - 判断是否存在

### 插入方法
- `Insert()` - 插入单条记录
- `InsertBatch()` - 批量插入
- `InsertBatchSplit()` - 分批批量插入

### 更新方法
- `UpdateById()` - 根据ID更新
- `Update()` - 条件更新

### 删除方法
- `Delete()` - 条件删除
- `DeleteById()` - 根据ID删除

### 其他方法
- `Tx()` - 设置事务
- `Table()` - 指定表名
- `WithContext()` - 设置上下文
- `And()` - AND条件组合
- `Or()` - OR条件组合
- `OrNest()` - 嵌套OR条件
- `AndIf()` - 条件判断AND
- `OrNestIf()` - 条件判断嵌套OR

## 注意事项

1. **模型定义**：确保结构体字段正确设置了数据库标签
2. **主键要求**：每个模型应至少有一个主键字段
3. **逻辑删除**：使用逻辑删除时，需要在数据库表中添加相应的删除标记字段
4. **事务安全**：在事务中操作时，所有相关操作都需要传递相同的事务对象
5. **性能考虑**：大量数据操作时建议使用批量操作方法

## 贡献

欢迎提交Issue和Pull Request来改进此项目。

## 许可证

MIT License