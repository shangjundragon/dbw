package sqlite_test

import (
	"database/sql"
	"encoding/json"
	"log"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/shangjundragon/dbw"
)

// 辅助函数：获取字符串指针
func getStringPtr(s string) *string {
	return &s
}

var (
	testConfig *dbw.Config
)

func init() {
	// 打开数据库连接（内存模式）
	db, err := sql.Open("sqlite", "test.db")
	if err != nil {
		log.Fatal(err)
	}

	testConfig = dbw.NewConfig(func(config *dbw.Config) {
		config.Db = db
		config.Debug = true
		config.DriverName = "sqlite"
	})

	// 创建测试表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS sys_user (
		id INTEGER PRIMARY KEY,
		username TEXT DEFAULT 'u',
		password TEXT DEFAULT 'p',
		nick_name TEXT,
		age INTEGER DEFAULT 0,
		amount REAL DEFAULT 0.0,
		create_time INTEGER,
		update_time INTEGER,
		del_flag TEXT DEFAULT '0'
	);
	
	CREATE TABLE IF NOT EXISTS product (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		price REAL,
		stock INTEGER DEFAULT 0,
		status INTEGER DEFAULT 1,
		create_time INTEGER,
		update_time INTEGER
	);
	
	CREATE TABLE IF NOT EXISTS order_info (
		id INTEGER PRIMARY KEY,
		order_no TEXT,
		user_id INTEGER,
		amount REAL,
		status INTEGER,
		create_time INTEGER,
		update_time INTEGER,
		del_flag TEXT DEFAULT '0'
	);
	`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("测试数据库初始化完成")
}

// User 测试结构体
type User struct {
	Id         int64   `dbw:"primaryKey"`
	Username   string  `dbw:"default:u"`
	Password   string  `dbw:"default:p"`
	NickName   *string `dbw:"column:nick_name"` // 指针类型支持 NULL
	Age        int     `dbw:"default:0"`
	Amount     float64 `dbw:"default:0.0"`
	CreateTime int64   `dbw:"autoCreateTime:milli"`
	UpdateTime int64   `dbw:"autoUpdateTime:milli"`
	DelFlag    string  `dbw:"tableLogic"`
}

func (User) TableName() string {
	return "sys_user"
}

// Product 测试结构体（无逻辑删除）
type Product struct {
	Id         int64 `dbw:"primaryKey"`
	Name       string
	Price      *float64 `dbw:"column:price"` // 指针类型，用于测试更新策略
	Stock      int      `dbw:"default:0"`
	Status     int      `dbw:"default:1"`
	CreateTime int64    `dbw:"autoCreateTime:milli"`
	UpdateTime int64    `dbw:"autoUpdateTime:milli"`
}

// OrderInfo 测试结构体（带逻辑删除）
type OrderInfo struct {
	Id         int64  `dbw:"primaryKey"`
	OrderNo    string `dbw:"column:order_no"`
	UserId     int64  `dbw:"column:user_id"`
	Amount     float64
	Status     int
	CreateTime int64  `dbw:"autoCreateTime:milli"`
	UpdateTime int64  `dbw:"autoUpdateTime:milli"`
	DelFlag    string `dbw:"tableLogic"`
}

func (OrderInfo) TableName() string {
	return "order_info"
}

// ==================== 插入测试 ====================

func TestInsert(t *testing.T) {
	nickName := "张三"
	user := &User{
		Username: "zhangsan",
		Password: "123456",
		NickName: &nickName,
		Age:      18,
	}

	result, err := dbw.New[User](dbw.WithConfig(testConfig)).Insert(user)
	if err != nil {
		t.Fatalf("插入失败：%v", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		t.Fatal("插入未影响任何行")
	}

	if user.Id == 0 {
		t.Fatal("雪花 ID 未生成")
	}

	log.Printf("✓ 单条插入成功，ID=%d, 受影响行数=%d", user.Id, rows)
}

func TestInsertBatch(t *testing.T) {
	users := []User{
		{Username: "user1", NickName: getStringPtr("用户 1"), Age: 20},
		{Username: "user2", NickName: getStringPtr("用户 2"), Age: 21},
		{Username: "user3", NickName: getStringPtr("用户 3"), Age: 22},
	}

	result, err := dbw.New[User](dbw.WithConfig(testConfig)).InsertBatch(users)
	if err != nil {
		t.Fatalf("批量插入失败：%v", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("✓ 批量插入成功，受影响行数=%d", rows)
}

func TestInsertBatchSplit(t *testing.T) {
	var users []User
	for i := 0; i < 15; i++ {
		users = append(users, User{
			Username: "batch_user_" + string(rune(i+'0')),
			NickName: getStringPtr("批量用户" + string(rune(i+'0'))),
			Age:      18 + i,
		})
	}

	results, err := dbw.New[User](dbw.WithConfig(testConfig)).InsertBatchSplit(users, 5)
	if err != nil {
		t.Fatalf("分批批量插入失败：%v", err)
	}

	log.Printf("✓ 分批批量插入成功，共%d批", len(results))
}

// ==================== 查询测试 ====================

func TestSelectById(t *testing.T) {
	// 先插入一条数据
	nickName := "test_select"
	user := &User{Username: "test_select", Age: 25, NickName: &nickName}
	dbw.New[User](dbw.WithConfig(testConfig)).Insert(user)

	// 根据 ID 查询
	result, err := dbw.New[User](dbw.WithConfig(testConfig)).SelectById(user.Id)
	if err != nil {
		t.Fatalf("根据 ID 查询失败：%v", err)
	}

	if result == nil {
		t.Fatal("查询结果为空")
	}

	if result.Username != "test_select" {
		t.Errorf("用户名错误，期望 test_select，实际 %s", result.Username)
	}

	log.Printf("✓ 根据 ID 查询成功：%+v", result)
}

func TestSelectOne(t *testing.T) {
	result, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Eq("username", "zhangsan").
		SelectOne()

	if err != nil {
		t.Fatalf("查询单条失败：%v", err)
	}

	if result == nil {
		t.Fatal("查询结果为空")
	}

	log.Printf("✓ 查询单条成功：%+v", result)
}

func TestSelectList(t *testing.T) {
	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Gt("age", 18).
		OrderByDesc("id").
		SelectList()

	if err != nil {
		t.Fatalf("查询列表失败：%v", err)
	}

	log.Printf("✓ 查询列表成功，共%d条记录", len(list))

	// 打印 JSON
	data, _ := json.Marshal(list)
	log.Printf("数据：%s", string(data))
}

func TestSelectPage(t *testing.T) {
	list, count, err := dbw.New[User](dbw.WithConfig(testConfig)).
		SelectPage(1, 5)

	if err != nil {
		t.Fatalf("分页查询失败：%v", err)
	}

	log.Printf("✓ 分页查询成功，总数=%d, 当前页=%d条", count, len(list))
}

func TestSelectWithConditions(t *testing.T) {
	// 测试多种条件组合
	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Eq("age", 20).
		Or().
		Eq("age", 21).
		OrderBy("age").
		SelectList()

	if err != nil {
		t.Fatalf("条件查询失败：%v", err)
	}

	log.Printf("✓ 条件查询成功，共%d条", len(list))
}

func TestSelectWithBetween(t *testing.T) {
	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Between("age", 20, 22).
		SelectList()

	if err != nil {
		t.Fatalf("BETWEEN 查询失败：%v", err)
	}

	log.Printf("✓ BETWEEN 查询成功，共%d条", len(list))
}

func TestSelectWithIn(t *testing.T) {
	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		In("age", 20, 21, 22).
		SelectList()

	if err != nil {
		t.Fatalf("IN 查询失败：%v", err)
	}

	log.Printf("✓ IN 查询成功，共%d条", len(list))
}

func TestSelectCount(t *testing.T) {
	count, err := dbw.New[User](dbw.WithConfig(testConfig)).Count()
	if err != nil {
		t.Fatalf("计数失败：%v", err)
	}

	log.Printf("✓ 总记录数：%d", count)
}

func TestSelectExist(t *testing.T) {
	exists, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Eq("username", "zhangsan").
		Exist()

	if err != nil {
		t.Fatalf("存在性检查失败：%v", err)
	}

	if !exists {
		t.Error("应该存在该用户")
	}

	log.Printf("✓ 存在性检查成功：exists=%v", exists)
}

func TestSelectWithGroupBy(t *testing.T) {
	type AgeCount struct {
		Age   int
		Count int64
	}

	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Select("age", "COUNT(*) as count").
		GroupBy("age").
		Having("COUNT(*) > ?", 0).
		SelectList()

	if err != nil {
		t.Fatalf("分组查询失败：%v", err)
	}

	log.Printf("✓ 分组查询成功，共%d组", len(list))
}

func TestSelectWithDistinct(t *testing.T) {
	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Distinct().
		Select("age").
		SelectList()

	if err != nil {
		t.Fatalf("去重查询失败：%v", err)
	}

	log.Printf("✓ 去重查询成功，共%d个不同年龄", len(list))
}

// ==================== 更新测试 ====================

func TestUpdateById(t *testing.T) {
	// 先插入一条数据
	nickName := "update_test"
	user := &User{Username: "update_test", Age: 30, NickName: &nickName}
	dbw.New[User](dbw.WithConfig(testConfig)).Insert(user)

	// 更新
	newNickName := "更新后的昵称"
	user.NickName = &newNickName
	user.Age = 31

	result, err := dbw.New[User](dbw.WithConfig(testConfig)).UpdateById(user)
	if err != nil {
		t.Fatalf("根据 ID 更新失败：%v", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("✓ 根据 ID 更新成功，受影响行数=%d", rows)
}

func TestUpdateWithMap(t *testing.T) {
	result, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Eq("username", "update_test").
		Update(map[string]any{
			"nick_name": "映射更新",
			"age":       32,
		})

	if err != nil {
		t.Fatalf("条件更新失败：%v", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("✓ 条件更新成功，受影响行数=%d", rows)
}

func TestUpdateWithPointerField(t *testing.T) {
	// 测试指针类型字段的更新
	price := 99.99
	product := &Product{
		Name:  "测试商品",
		Price: &price,
		Stock: 100,
	}

	_, err := dbw.New[Product](dbw.WithConfig(testConfig)).Insert(product)
	if err != nil {
		t.Fatalf("插入商品失败：%v", err)
	}

	// 只更新价格字段
	newPrice := 199.99
	product.Price = &newPrice

	result, err := dbw.New[Product](dbw.WithConfig(testConfig)).UpdateById(product)
	if err != nil {
		t.Fatalf("更新商品失败：%v", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("✓ 指针字段更新成功，受影响行数=%d", rows)
}

// ==================== 删除测试 ====================

func TestDeleteById(t *testing.T) {
	// 先插入一条数据
	nickName := "delete_test"
	user := &User{Username: "delete_test", Age: 25, NickName: &nickName}
	dbw.New[User](dbw.WithConfig(testConfig)).Insert(user)

	// 删除（逻辑删除）
	result, err := dbw.New[User](dbw.WithConfig(testConfig)).DeleteById(user.Id)
	if err != nil {
		t.Fatalf("根据 ID 删除失败：%v", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("✓ 根据 ID 删除成功，受影响行数=%d", rows)

	// 验证逻辑删除后查询不到
	count, _ := dbw.New[User](dbw.WithConfig(testConfig)).
		Eq("id", user.Id).
		Count()

	if count != 0 {
		t.Error("逻辑删除后应该查询不到该记录")
	}

	log.Printf("✓ 逻辑删除验证通过")
}

func TestDeleteByIds(t *testing.T) {
	// 先插入几条数据
	users := []User{
		{Username: "del_user1", Age: 20},
		{Username: "del_user2", Age: 21},
	}
	dbw.New[User](dbw.WithConfig(testConfig)).InsertBatch(users)

	// 使用 In 方法直接删除
	result, err := dbw.New[User](dbw.WithConfig(testConfig)).
		In("id", users[0].Id, users[1].Id).
		Delete()
	if err != nil {
		t.Fatalf("批量删除失败：%v", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("✓ 批量删除成功，受影响行数=%d", rows)
}

func TestDeleteWithCondition(t *testing.T) {
	result, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Lt("age", 20).
		Delete()

	if err != nil {
		t.Fatalf("条件删除失败：%v", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("✓ 条件删除成功，受影响行数=%d", rows)
}

// ==================== 事务测试 ====================

func TestTransaction(t *testing.T) {
	err := dbw.ExecuteTx(func(tx *sql.Tx) error {
		// 在事务中插入用户
		nickName := "tx_user"
		user := &User{Username: "tx_user", Age: 28, NickName: &nickName}
		_, err := dbw.New[User](dbw.WithConfig(testConfig), dbw.WithTx(tx)).Insert(user)
		if err != nil {
			return err
		}

		// 在事务中插入订单
		order := &OrderInfo{
			OrderNo: "ORDER_001",
			UserId:  user.Id,
			Amount:  100.0,
			Status:  1,
		}
		_, err = dbw.New[OrderInfo](dbw.WithConfig(testConfig), dbw.WithTx(tx)).Insert(order)
		if err != nil {
			return err
		}

		log.Printf("✓ 事务中插入用户和订单成功")
		return nil
	}, testConfig.Db)

	if err != nil {
		t.Fatalf("事务执行失败：%v", err)
	}

	log.Printf("✓ 事务提交成功")
}

func TestTransactionRollback(t *testing.T) {
	// 先创建一个会失败的表约束
	_, err := testConfig.Db.Exec("CREATE TABLE IF NOT EXISTS unique_user (id INTEGER PRIMARY KEY, username TEXT UNIQUE)")
	if err != nil {
		t.Fatalf("创建表失败：%v", err)
	}

	err = dbw.ExecuteTx(func(tx *sql.Tx) error {
		// 插入第一条
		type UniqueUser struct {
			Id       int64 `dbw:"primaryKey"`
			Username string
		}

		user1 := &UniqueUser{Username: "unique_user1"}
		_, err := dbw.New[UniqueUser](dbw.WithConfig(testConfig), dbw.WithTx(tx)).Insert(user1)
		if err != nil {
			return err
		}

		// 故意制造错误（插入重复的用户名触发唯一约束）
		user2 := &UniqueUser{Username: "unique_user1"} // 重复的用户名
		_, err = dbw.New[UniqueUser](dbw.WithConfig(testConfig), dbw.WithTx(tx)).Insert(user2)
		if err != nil {
			// 返回错误，触发回滚
			return err
		}

		return nil
	}, testConfig.Db)

	if err == nil {
		t.Error("事务应该回滚")
	} else {
		log.Printf("✓ 事务回滚成功，错误：%v", err)
	}
}

// ==================== 高级功能测试 ====================

func TestWhereIf(t *testing.T) {
	age := 20
	city := ""

	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		WhereIf(age > 0, "age > ?", age).
		EqIf(city != "", "city", city).
		SelectList()

	if err != nil {
		t.Fatalf("条件判断查询失败：%v", err)
	}

	log.Printf("✓ WhereIf 查询成功，共%d条", len(list))
}

func TestNestedWhere(t *testing.T) {
	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Eq("age", 20).
		And(func(w *dbw.DbWrapper[User]) {
			w.Eq("age", 20).Or().Eq("age", 21)
		}).
		SelectList()

	if err != nil {
		t.Fatalf("嵌套条件查询失败：%v", err)
	}

	log.Printf("✓ 嵌套条件查询成功，共%d条", len(list))
}

func TestUpdateStrategy(t *testing.T) {
	// 测试总是更新的策略
	product := &Product{
		Name:  "策略测试商品",
		Stock: 50,
	}

	_, err := dbw.New[Product](dbw.WithConfig(testConfig)).Insert(product)
	if err != nil {
		t.Fatalf("插入商品失败：%v", err)
	}

	// Stock 为 0 时也应该更新（如果有 default 标签或 always 策略）
	product.Stock = 0
	result, err := dbw.New[Product](dbw.WithConfig(testConfig)).UpdateById(product)
	if err != nil {
		t.Fatalf("更新商品失败：%v", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("✓ 零值更新成功，受影响行数=%d", rows)
}

func TestNullAndNotNull(t *testing.T) {
	// 测试 IS NULL 和 IS NOT NULL
	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		NotNull("nick_name").
		SelectList()

	if err != nil {
		t.Fatalf("IS NOT NULL 查询失败：%v", err)
	}

	log.Printf("✓ IS NOT NULL 查询成功，共%d条", len(list))
}

func TestLikeQuery(t *testing.T) {
	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Like("username", "%user%").
		SelectList()

	if err != nil {
		t.Fatalf("LIKE 查询失败：%v", err)
	}

	log.Printf("✓ LIKE 查询成功，共%d条", len(list))
}

func TestOrderByMultiple(t *testing.T) {
	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		OrderByDesc("age").
		OrderBy("id").
		SelectList()

	if err != nil {
		t.Fatalf("多字段排序失败：%v", err)
	}

	log.Printf("✓ 多字段排序成功，共%d条", len(list))
}

func TestContextTimeout(t *testing.T) {
	ctx, cancel := dbw.GetContextWithTimeout(5 * time.Second)
	defer cancel()

	list, err := dbw.New[User](dbw.WithConfig(testConfig)).
		WithContext(ctx).
		SelectList()

	if err != nil {
		t.Fatalf("上下文查询失败：%v", err)
	}

	log.Printf("✓ 上下文查询成功，共%d条", len(list))
}

// ==================== 边界情况测试 ====================

func TestEmptySlice(t *testing.T) {
	users := []User{}

	_, err := dbw.New[User](dbw.WithConfig(testConfig)).InsertBatch(users)
	if err == nil {
		t.Error("空切片批量插入应该报错")
	}

	log.Printf("✓ 空切片校验通过：%v", err)
}

func TestNilPointer(t *testing.T) {
	_, err := dbw.New[User](dbw.WithConfig(testConfig)).Insert(nil)
	if err == nil {
		t.Error("nil 指针插入应该报错")
	}

	log.Printf("✓ nil 指针校验通过：%v", err)
}

func TestUpdateWithoutWhere(t *testing.T) {
	_, err := dbw.New[User](dbw.WithConfig(testConfig)).
		Update(map[string]any{"age": 99})

	if err == nil {
		t.Error("不带 WHERE 条件的更新应该报错")
	}

	log.Printf("✓ 更新安全校验通过：%v", err)
}

func TestDeleteWithoutWhere(t *testing.T) {
	_, err := dbw.New[User](dbw.WithConfig(testConfig)).Delete()

	if err == nil {
		t.Error("不带 WHERE 条件的删除应该报错")
	}

	log.Printf("✓ 删除安全校验通过：%v", err)
}

// ==================== 性能测试 ====================

func BenchmarkInsert(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nickName := "bench"
		user := &User{Username: "bench_user", Age: 25, NickName: &nickName}
		_, err := dbw.New[User](dbw.WithConfig(testConfig)).Insert(user)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelectById(b *testing.B) {
	// 先插入一条数据
	nickName := "bench_select"
	user := &User{Username: "bench_select", Age: 25, NickName: &nickName}
	dbw.New[User](dbw.WithConfig(testConfig)).Insert(user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dbw.New[User](dbw.WithConfig(testConfig)).SelectById(user.Id)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkInsertBatch(b *testing.B) {
	users := make([]User, 100)
	for i := 0; i < 100; i++ {
		nickName := "batch"
		users[i] = User{Username: "batch_bench", Age: i, NickName: &nickName}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dbw.New[User](dbw.WithConfig(testConfig)).InsertBatch(users)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestSelect(t *testing.T) {
	userDbw := dbw.New[User](dbw.WithConfig(testConfig))
	list, err := userDbw.Eq("username", "zhangsan").OrNest(func(d *dbw.DbWrapper[User]) {
		d.Eq("age", 20)
		d.Or().Eq("age", 21)
	}).SelectList()
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	log.Printf("✓ 查询成功，共%d条", len(list))
}
