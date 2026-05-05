package sqlite_test

import (
	"errors"
	"testing"

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
