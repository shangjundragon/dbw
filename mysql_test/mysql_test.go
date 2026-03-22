package mysql_test

import (
	"database/sql"
	"encoding/json"

	"fmt"
	"log"

	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shangjundragon/dbw"
)

var config *dbw.Config

func init() {
	// 数据库连接信息
	// 格式: "用户名:密码@tcp(主机:端口)/数据库名称?charset=utf8&parseTime=True&loc=Local"
	// 注意：parseTime=True 用于将数据库中的时间类型解析为Go的时间类型
	// loc=Local 设置时区为本地时区
	dsn := "root:123456@tcp(192.168.31.52:3306)/test?charset=utf8&parseTime=True&loc=Local"

	// 打开数据库连接
	db, err := sql.Open("mysql", dsn)

	// 尝试连接数据库
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("成功连接到MySQL数据库")
	config = dbw.NewConfig(func(config *dbw.Config) {
		config.Db = db
		config.Debug = true
		config.DriverName = "mysql"
	})

}

type Student struct {
	Id         int64
	Username   string
	NickName   string
	Age        int       `dbw:"default:1"`
	CreateTime time.Time `dbw:"autoCreateTime"`
	UpdateTime time.Time `dbw:"autoUpdateTime"`
	DelFlag    string    `dbw:"tableLogic"`
}

func TestMysqlInsert(t *testing.T) {
	affected, err := dbw.New[Student](dbw.WithConfig(config)).Insert(&Student{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("成功 affected: %v", affected)
}

func TestMysqlInsertBatch(t *testing.T) {
	var students []Student
	students = append(students, Student{Username: "test1", NickName: "test1"})
	students = append(students, Student{Username: "test2", NickName: "test2"})
	students = append(students, Student{Username: "test3", NickName: "test3"})
	students = append(students, Student{Username: "test1", NickName: "test4"})

	err := dbw.ExecuteTx(func(tx *sql.Tx) error {
		affected, err := dbw.New[Student](dbw.WithConfig(config)).Tx(tx).InsertBatchSplit(students, 2)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("成功 affected: %v", affected)
		return nil
	}, config.Db)

	if err != nil {
		log.Fatal(err)
	}
}

func TestMysqlUpdate(t *testing.T) {
	one, err := dbw.New[Student](dbw.WithConfig(config)).SelectById(11)
	if err != nil {
		log.Fatal(err)
	}
	marshal, err := json.Marshal(one)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("成功 %v", string(marshal))
	one.Username = "test2"
	affected, err := dbw.New[Student](dbw.WithConfig(config)).UpdateById(&one)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("成功 affected: %v", affected)
}

func TestMysqlSelect(t *testing.T) {

	records, count, err := dbw.New[Student](dbw.WithConfig(config)).Select("id").SelectPage(1, 10)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("成功 count=%d", count)
	jsonData, err := json.Marshal(records)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("成功 %v", string(jsonData))

}
