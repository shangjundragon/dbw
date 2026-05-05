package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/shangjundragon/dbw"
)

func TestHooksInstanceBeforeInsert(t *testing.T) {
	tag := ""
	wrapper := dbw.New[User](dbw.WithConfig(testConfig), dbw.WithHooks(func(h *dbw.Hooks[User]) {
		h.BeforeInsert = func(q *dbw.DbWrapper[User], data *User) error {
			data.Username = "hook_" + data.Username
			data.NickName = &tag
			return nil
		}
	}))

	data := &User{Username: "test_before_insert", Age: 20}
	_, err := wrapper.Insert(data)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if data.Username != "hook_test_before_insert" {
		t.Errorf("BeforeInsert hook not called, got Username=%q", data.Username)
	}

	// Clean up
	cleanWrapper := dbw.New[User](dbw.WithConfig(testConfig))
	_, err = cleanWrapper.Eq("id", data.Id).Delete()
	if err != nil {
		t.Fatalf("cleanup delete failed: %v", err)
	}
}

func TestHooksErrorPropagation(t *testing.T) {
	wantErr := errors.New("hook error")
	wrapper := dbw.New[User](dbw.WithConfig(testConfig), dbw.WithHooks(func(h *dbw.Hooks[User]) {
		h.BeforeInsert = func(q *dbw.DbWrapper[User], data *User) error {
			return wantErr
		}
	}))

	data := &User{Username: "test_error", Age: 20}
	_, err := wrapper.Insert(data)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected hook error, got %v", err)
	}
}

func TestHooksAfterQuery(t *testing.T) {
	// Insert a user
	wrapper := dbw.New[User](dbw.WithConfig(testConfig))
	data := &User{Username: "test_after_query", Age: 20}
	_, err := wrapper.Insert(data)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	// Query with AfterQuery hook
	wrapper2 := dbw.New[User](dbw.WithConfig(testConfig), dbw.WithHooks(func(h *dbw.Hooks[User]) {
		h.AfterQuery = func(q *dbw.DbWrapper[User], user *User) error {
			user.Username = "modified_" + user.Username
			return nil
		}
	}))

	result, err := wrapper2.Eq("id", data.Id).FindOne()
	if err != nil {
		t.Fatalf("findone failed: %v", err)
	}
	if result.Username != "modified_test_after_query" {
		t.Errorf("AfterQuery hook not called, got Username=%q", result.Username)
	}

	// Clean up
	_, err = wrapper.Eq("id", data.Id).Delete()
	if err != nil {
		t.Fatalf("cleanup delete failed: %v", err)
	}
}

func TestHooksBeforeUpdate(t *testing.T) {
	wrapper := dbw.New[User](dbw.WithConfig(testConfig))
	data := &User{Username: "test_before_update", Age: 20}
	_, err := wrapper.Insert(data)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	wrapper2 := dbw.New[User](dbw.WithConfig(testConfig), dbw.WithHooks(func(h *dbw.Hooks[User]) {
		h.BeforeUpdate = func(q *dbw.DbWrapper[User], data *User) error {
			data.NickName = &data.Username
			return nil
		}
	}))

	data.Username = "updated_name"
	_, err = wrapper2.UpdateById(data)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if data.NickName == nil || *data.NickName != "updated_name" {
		t.Errorf("BeforeUpdate hook not called, got NickName=%v", data.NickName)
	}

	// Clean up
	_, err = wrapper.Eq("id", data.Id).Delete()
	if err != nil {
		t.Fatalf("cleanup delete failed: %v", err)
	}
}

func TestHooksBeforeUpdateMap(t *testing.T) {
	wrapper := dbw.New[User](dbw.WithConfig(testConfig))
	data := &User{Username: "test_before_update_map", Age: 20}
	_, err := wrapper.Insert(data)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	wrapper2 := dbw.New[User](dbw.WithConfig(testConfig), dbw.WithHooks(func(h *dbw.Hooks[User]) {
		h.BeforeUpdateMap = func(q *dbw.DbWrapper[User], values map[string]any) error {
			values["nick_name"] = "map_updated"
			return nil
		}
	}))

	_, err = wrapper2.Eq("id", data.Id).Update(map[string]any{"age": 25})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	result, _ := wrapper.Eq("id", data.Id).FindOne()
	if result.NickName == nil || *result.NickName != "map_updated" {
		t.Errorf("BeforeUpdateMap hook not called, got NickName=%v", result.NickName)
	}

	// Clean up
	_, err = wrapper.Eq("id", data.Id).Delete()
	if err != nil {
		t.Fatalf("cleanup delete failed: %v", err)
	}
}

func TestHooksBeforeDelete(t *testing.T) {
	var deleteCalled bool
	wrapper := dbw.New[User](dbw.WithConfig(testConfig))
	data := &User{Username: "test_before_delete", Age: 20}
	_, err := wrapper.Insert(data)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	wrapper2 := dbw.New[User](dbw.WithConfig(testConfig), dbw.WithHooks(func(h *dbw.Hooks[User]) {
		h.BeforeDelete = func(q *dbw.DbWrapper[User]) error {
			deleteCalled = true
			return nil
		}
	}))

	_, err = wrapper2.Eq("id", data.Id).Delete()
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if !deleteCalled {
		t.Error("BeforeDelete hook not called")
	}
}

func TestHooksGlobalAndInstance(t *testing.T) {
	dbw.RegisterHooks[User](func(h *dbw.Hooks[User]) {
		h.BeforeInsert = func(q *dbw.DbWrapper[User], data *User) error {
			data.Username = "global_" + data.Username
			return nil
		}
	})
	// Reset after test
	defer dbw.RegisterHooks[User](func(h *dbw.Hooks[User]) {
		h.BeforeInsert = nil
	})

	// Instance extends global
	wrapper := dbw.New[User](dbw.WithConfig(testConfig), dbw.WithHooks(func(h *dbw.Hooks[User]) {
		h.BeforeInsert = func(q *dbw.DbWrapper[User], data *User) error {
			data.Username = data.Username + "_instance"
			return nil
		}
	}))

	data := &User{Username: "test", Age: 20}
	_, err := wrapper.Insert(data)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	// Global runs first, then instance: global_ + test + _instance
	if data.Username != "global_test_instance" {
		t.Errorf("hooks order wrong, got Username=%q", data.Username)
	}

	// Clean up
	cleanWrapper := dbw.New[User](dbw.WithConfig(testConfig))
	_, err = cleanWrapper.Eq("id", data.Id).Delete()
	if err != nil {
		t.Fatalf("cleanup delete failed: %v", err)
	}
}

func TestEntityHookBeforeInsert(t *testing.T) {
	dbw.RegisterEntityHook(func(ctx context.Context, point dbw.HookPoint, entity any) error {
		if point != dbw.HookBeforeInsert || entity == nil {
			return nil
		}
		v := reflect.ValueOf(entity).Elem()
		tp := v.Type()
		for i := 0; i < tp.NumField(); i++ {
			tagMap := dbw.ResolveDbwTag(tp.Field(i).Tag.Get("dbw"))
			if tagMap["autoCreateUser"] == "true" {
				v.Field(i).Set(reflect.ValueOf(int64(42)))
			}
		}
		return nil
	})

	type EntityWithCreateBy struct {
		ID       int64 `dbw:"primaryKey"`
		Name     string
		CreateBy int64 `dbw:"autoCreateUser"`
	}

	db, cfg := newTestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS entity_with_create_by (
		id INTEGER PRIMARY KEY,
		name TEXT,
		create_by INTEGER
	)`)
	if err != nil {
		t.Fatal(err)
	}

	wrapper := dbw.New[EntityWithCreateBy](dbw.WithConfig(cfg))
	data := &EntityWithCreateBy{Name: "test_create"}
	_, err = wrapper.Insert(data)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if data.CreateBy != 42 {
		t.Errorf("EntityHook BeforeInsert not called, got CreateBy=%d", data.CreateBy)
	}
}

func TestEntityHookBeforeUpdate(t *testing.T) {
	dbw.RegisterEntityHook(func(ctx context.Context, point dbw.HookPoint, entity any) error {
		if point != dbw.HookBeforeUpdate || entity == nil {
			return nil
		}
		v := reflect.ValueOf(entity).Elem()
		tp := v.Type()
		for i := 0; i < tp.NumField(); i++ {
			tagMap := dbw.ResolveDbwTag(tp.Field(i).Tag.Get("dbw"))
			if tagMap["autoUpdateUser"] == "true" {
				v.Field(i).Set(reflect.ValueOf(int64(99)))
			}
		}
		return nil
	})

	type EntityWithUpdateBy struct {
		ID       int64 `dbw:"primaryKey"`
		Name     string
		UpdateBy int64 `dbw:"autoUpdateUser"`
	}

	db, cfg := newTestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS entity_with_update_by (
		id INTEGER PRIMARY KEY,
		name TEXT,
		update_by INTEGER
	)`)
	if err != nil {
		t.Fatal(err)
	}

	wrapper := dbw.New[EntityWithUpdateBy](dbw.WithConfig(cfg))
	data := &EntityWithUpdateBy{Name: "test_update"}
	_, err = wrapper.Insert(data)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	data.Name = "updated"
	_, err = wrapper.UpdateById(data)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if data.UpdateBy != 99 {
		t.Errorf("EntityHook BeforeUpdate not called, got UpdateBy=%d", data.UpdateBy)
	}
}

func TestEntityHookErrorPropagation(t *testing.T) {
	wantErr := errors.New("entity hook error")

	type SimpleEntity struct {
		ID   int64 `dbw:"primaryKey"`
		Name string
	}

	dbw.RegisterEntityHook(func(ctx context.Context, point dbw.HookPoint, entity any) error {
		if point == dbw.HookBeforeInsert {
			if _, ok := entity.(*SimpleEntity); ok {
				return wantErr
			}
		}
		return nil
	})

	db, cfg := newTestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS simple_entity (
		id INTEGER PRIMARY KEY,
		name TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	wrapper := dbw.New[SimpleEntity](dbw.WithConfig(cfg))
	_, err = wrapper.Insert(&SimpleEntity{Name: "test"})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected entity hook error, got %v", err)
	}
}

func TestEntityHookOrderBeforeHooksT(t *testing.T) {
	steps := ""
	dbw.RegisterEntityHook(func(ctx context.Context, point dbw.HookPoint, entity any) error {
		if point == dbw.HookBeforeInsert {
			steps += ":entity"
		}
		return nil
	})

	dbw.RegisterHooks[User](func(h *dbw.Hooks[User]) {
		h.BeforeInsert = func(q *dbw.DbWrapper[User], data *User) error {
			steps += ":hooks_t"
			return nil
		}
	})
	defer dbw.RegisterHooks[User](func(h *dbw.Hooks[User]) {
		h.BeforeInsert = nil
	})

	wrapper := dbw.New[User](dbw.WithConfig(testConfig))
	data := &User{Username: "test_order", Age: 20}
	_, err := wrapper.Insert(data)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if steps != ":entity:hooks_t" {
		t.Errorf("wrong order, got %q", steps)
	}

	cleanWrapper := dbw.New[User](dbw.WithConfig(testConfig))
	_, err = cleanWrapper.Eq("id", data.Id).Delete()
	if err != nil {
		t.Fatalf("cleanup delete failed: %v", err)
	}
}

func newTestDB(t *testing.T) (*sql.DB, *dbw.Config) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:hooks_entity_test.db?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	cfg := dbw.NewConfig(func(c *dbw.Config) {
		c.Db = db
		c.DriverName = "sqlite"
	})
	return db, cfg
}
