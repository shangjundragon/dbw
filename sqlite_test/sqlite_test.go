package sqlite_test

import (
	"database/sql"
	"encoding/json"
	"log"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/shangjundragon/dbw"
)

var config *dbw.Config

func init() {
	db, err := sql.Open("sqlite", "test.db")
	if err != nil {
		log.Fatal(err)
	}
	config = dbw.NewConfig(func(config *dbw.Config) {
		config.Db = db
		config.Debug = true
		config.DriverName = "sqlite"
	})

}

type SysUser struct {
	Id         int64  `dbw:"idType:autoIncrement"`
	Username   string `dbw:"default:u"`
	Password   string `dbw:"default:p"`
	Age        int    `dbw:"default:0"`
	CreateTime int64  `dbw:"autoCreateTime:milli"`
	UpdateTime int64  `dbw:"autoUpdateTime:milli"`
	DelFlag    string `dbw:"tableLogic:true"`
}

/*func (SysUser) TableName() string {
	return "sys_user_s"
}*/

func TestSelect(t *testing.T) {
	list, err := dbw.New[SysUser](dbw.WithConfig(config)).SelectList()
	if err != nil {
		log.Fatal(err)
	}
	marshal, err := json.Marshal(list)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(marshal))
}

func TestInsert(t *testing.T) {
	user := &SysUser{}
	affected, err := dbw.New[SysUser](dbw.WithConfig(config)).Insert(user)

	if err != nil {
		log.Fatalf("错误 err:%v", err)
	}
	marshal, _ := json.Marshal(user)
	log.Printf("成功 affected rows: %d\n", affected)
	log.Println(string(marshal))
}

func TestUpdateById(t *testing.T) {
	one, err := dbw.New[SysUser](dbw.WithConfig(config)).SelectById(3)
	if err != nil {
		log.Fatal(err)
	}
	jsonStr, err := json.Marshal(one)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(jsonStr))
	one.Username = ""
	affected, err := dbw.New[SysUser](dbw.WithConfig(config)).UpdateById(&one)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("affected rows: %d\n", affected)
}

func TestUpdate(t *testing.T) {
	//New[SysUser]().Insert(&SysUser{Username: GetStringPtr("test"), Password: GetStringPtr("test")})
	affected, err := dbw.New[SysUser](dbw.WithConfig(config)).Eq("id", 1).Update(map[string]any{"username": "zs"})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("affected rows: %d\n", affected)
}

func TestDelete(t *testing.T) {
	//New[SysUser]().Insert(&SysUser{Username: GetStringPtr("test"), Password: GetStringPtr("test")})
	affected, err := dbw.New[SysUser](dbw.WithConfig(config)).DeleteById(1)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("affected rows: %d\n", affected)
}
