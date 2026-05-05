# Hooks 生命周期钩子 — 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 DBW 添加全局+实例级生命周期钩子，支持 BeforeInsert、AfterInsert、BeforeUpdate、BeforeUpdateMap、AfterUpdate、BeforeDelete、AfterDelete、AfterQuery

**Architecture:** 新增 `Hooks[T]` 结构体 + `RegisterHooks[T]` 全局注册 + `WithHooks` 实例选项 + 8个 `callBeforeXxx`/`callAfterXxx` 辅助方法，hooks 在 `DbWrapper` 中存储为 `any` 以兼容 `Options` 类型

**Tech Stack:** Go 1.21 generics, reflect, sync.Map

---

### Task 1: 新建 hooks.go — Hooks 类型、全局注册、WithHooks 选项

**Files:**
- Create: `hooks.go`
- Modify: `wrapper.go:11` (add `hooks any` 字段)

- [ ] **Step 1: 在 hooks.go 中定义 Hooks[T] 类型**

```go
package dbw

import (
    "context"
    "database/sql"
    "reflect"
    "sync"
)

type Hooks[T any] struct {
    BeforeInsert    func(ctx context.Context, data *T) error
    AfterInsert     func(ctx context.Context, data *T, result sql.Result) error
    BeforeUpdate    func(ctx context.Context, data *T) error
    BeforeUpdateMap func(ctx context.Context, values map[string]any) error
    AfterUpdate     func(ctx context.Context, result sql.Result) error
    BeforeDelete    func(ctx context.Context) error
    AfterDelete     func(ctx context.Context, result sql.Result) error
    AfterQuery      func(ctx context.Context, data *T) error
}

var globalHooks sync.Map

// typeKey returns the reflect.Type key for type T (Go 1.21 compatible).
func typeKey[T any]() reflect.Type {
    return reflect.TypeOf((*T)(nil)).Elem()
}

func RegisterHooks[T any](fn func(h *Hooks[T])) {
    key := typeKey[T]()
    raw, _ := globalHooks.LoadOrStore(key, &Hooks[T]{})
    fn(raw.(*Hooks[T]))
}

func getGlobalHooks[T any]() *Hooks[T] {
    key := typeKey[T]()
    if raw, ok := globalHooks.Load(key); ok {
        return raw.(*Hooks[T])
    }
    return nil
}

func WithHooks[T any](fn func(h *Hooks[T])) Options {
    h := &Hooks[T]{}
    fn(h)
    return func(opts *DbWrapper[any]) {
        opts.hooks = h
    }
}
```

- [ ] **Step 2: 在 wrapper.go DbWrapper[T] 中加 hooks any 字段**

在 `wrapper.go:27`（`meta *structMeta` 之后）添加：

```go
hooks     any
```

- [ ] **Step 3: 验证编译通过**

Run: `go build ./...`
Expected: 编译成功

---

### Task 2: wrapper.go — Clone/Clean/Reset 传播 hooks

**Files:**
- Modify: `wrapper.go:86-141`

- [ ] **Step 1: 修改 Reset 方法**

当前 `Reset` 没有 hooks 字段，不需要传 hooks 过去（按设计 Reset 重置 hooks 为 nil）。但需要保持不变。无需修改。

- [ ] **Step 2: 修改 Clone 方法**

Clone 是浅拷贝结构体，`hooks any` 字段自动被复制（指针共享）。无需显式修改。

- [ ] **Step 3: 修改 Clean 方法**

Clean 创建新结构体，显式列出要保留的字段。当前没有 hooks，也不需要在 Clean 中保留 hooks（Clean 最小化拷贝，hooks 带过去不合适）。

无需修改。

---

### Task 3: hooks.go — 8个 callXxx 辅助方法

**Files:**
- Modify: `hooks.go`

- [ ] **Step 1: 添加 callBeforeInsert**

```go
func (q *DbWrapper[T]) callBeforeInsert(ctx context.Context, data *T) error {
    if gh := getGlobalHooks[T](); gh != nil && gh.BeforeInsert != nil {
        if err := gh.BeforeInsert(ctx, data); err != nil {
            return err
        }
    }
    if q.hooks != nil {
        if h := q.hooks.(*Hooks[T]); h.BeforeInsert != nil {
            if err := h.BeforeInsert(ctx, data); err != nil {
                return err
            }
        }
    }
    return nil
}
```

- [ ] **Step 2: 添加 callAfterInsert**

```go
func (q *DbWrapper[T]) callAfterInsert(ctx context.Context, data *T, result sql.Result) error {
    if gh := getGlobalHooks[T](); gh != nil && gh.AfterInsert != nil {
        if err := gh.AfterInsert(ctx, data, result); err != nil {
            return err
        }
    }
    if q.hooks != nil {
        if h := q.hooks.(*Hooks[T]); h.AfterInsert != nil {
            if err := h.AfterInsert(ctx, data, result); err != nil {
                return err
            }
        }
    }
    return nil
}
```

- [ ] **Step 3: 添加 callBeforeUpdate**

```go
func (q *DbWrapper[T]) callBeforeUpdate(ctx context.Context, data *T) error {
    if gh := getGlobalHooks[T](); gh != nil && gh.BeforeUpdate != nil {
        if err := gh.BeforeUpdate(ctx, data); err != nil {
            return err
        }
    }
    if q.hooks != nil {
        if h := q.hooks.(*Hooks[T]); h.BeforeUpdate != nil {
            if err := h.BeforeUpdate(ctx, data); err != nil {
                return err
            }
        }
    }
    return nil
}
```

- [ ] **Step 4: 添加 callBeforeUpdateMap**

```go
func (q *DbWrapper[T]) callBeforeUpdateMap(ctx context.Context, values map[string]any) error {
    if gh := getGlobalHooks[T](); gh != nil && gh.BeforeUpdateMap != nil {
        if err := gh.BeforeUpdateMap(ctx, values); err != nil {
            return err
        }
    }
    if q.hooks != nil {
        if h := q.hooks.(*Hooks[T]); h.BeforeUpdateMap != nil {
            if err := h.BeforeUpdateMap(ctx, values); err != nil {
                return err
            }
        }
    }
    return nil
}
```

- [ ] **Step 5: 添加 callAfterUpdate**

```go
func (q *DbWrapper[T]) callAfterUpdate(ctx context.Context, result sql.Result) error {
    if gh := getGlobalHooks[T](); gh != nil && gh.AfterUpdate != nil {
        if err := gh.AfterUpdate(ctx, result); err != nil {
            return err
        }
    }
    if q.hooks != nil {
        if h := q.hooks.(*Hooks[T]); h.AfterUpdate != nil {
            if err := h.AfterUpdate(ctx, result); err != nil {
                return err
            }
        }
    }
    return nil
}
```

- [ ] **Step 6: 添加 callBeforeDelete**

```go
func (q *DbWrapper[T]) callBeforeDelete(ctx context.Context) error {
    if gh := getGlobalHooks[T](); gh != nil && gh.BeforeDelete != nil {
        if err := gh.BeforeDelete(ctx); err != nil {
            return err
        }
    }
    if q.hooks != nil {
        if h := q.hooks.(*Hooks[T]); h.BeforeDelete != nil {
            if err := h.BeforeDelete(ctx); err != nil {
                return err
            }
        }
    }
    return nil
}
```

- [ ] **Step 7: 添加 callAfterDelete**

```go
func (q *DbWrapper[T]) callAfterDelete(ctx context.Context, result sql.Result) error {
    if gh := getGlobalHooks[T](); gh != nil && gh.AfterDelete != nil {
        if err := gh.AfterDelete(ctx, result); err != nil {
            return err
        }
    }
    if q.hooks != nil {
        if h := q.hooks.(*Hooks[T]); h.AfterDelete != nil {
            if err := h.AfterDelete(ctx, result); err != nil {
                return err
            }
        }
    }
    return nil
}
```

- [ ] **Step 8: 添加 callAfterQuery**

```go
func (q *DbWrapper[T]) callAfterQuery(ctx context.Context, data *T) error {
    if gh := getGlobalHooks[T](); gh != nil && gh.AfterQuery != nil {
        if err := gh.AfterQuery(ctx, data); err != nil {
            return err
        }
    }
    if q.hooks != nil {
        if h := q.hooks.(*Hooks[T]); h.AfterQuery != nil {
            if err := h.AfterQuery(ctx, data); err != nil {
                return err
            }
        }
    }
    return nil
}
```

- [ ] **Step 9: 验证编译**

Run: `go build ./...`
Expected: 编译成功

---

### Task 4: insert.go — 集成 Insert 钩子

**Files:**
- Modify: `insert.go:48-97`

- [ ] **Step 1: 在 Insert 中调用 BeforeInsert 和 AfterInsert**

在 `insert.go:52`（`generatedId, err := q.beforeInsert(data)`）之后，columns 构建之前，加上：

```go
    if err := q.callBeforeInsert(q.ctx, data); err != nil {
        return nil, err
    }
```

在 `insert.go:96`（`return result, nil`）之前，加上：

```go
    if err := q.callAfterInsert(q.ctx, data, result); err != nil {
        return nil, err
    }
```

注意：InsertBatch 也应该触发 BeforeInsert/AfterInsert（按设计 InsertBatch 最终也走 Insert 语义）。但 InsertBatch 目前不经过 Insert()，而是直接构建 SQL。为保持简单，InsertBatch 暂不触发钩子（用户可用 InsertBatchSplit 逐一插入来触发）。

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: 编译成功

---

### Task 5: update.go — 集成 Update 钩子

**Files:**
- Modify: `update.go:11-85`（UpdateById）和 `update.go:88-115`（Update）

- [ ] **Step 1: UpdateById 中调用 BeforeUpdate 和 AfterUpdate**

在 `update.go:19`（`elem := reflect.ValueOf(data).Elem()`）之后，SET 循环之前，加上：

```go
    if err := q.callBeforeUpdate(q.ctx, data); err != nil {
        return nil, err
    }
```

在 `update.go:83`（`return result, nil`）之前，加上：

```go
    if err := q.callAfterUpdate(q.ctx, result); err != nil {
        return nil, err
    }
```

- [ ] **Step 2: Update(map) 中调用 BeforeUpdateMap 和 AfterUpdate**

在 `update.go:94`（`if len(q.wheres) == 0` 检查之后）之前，加上：

```go
    if err := q.callBeforeUpdateMap(q.ctx, values); err != nil {
        return nil, err
    }
```

在 `update.go:113`（`return result, nil`）之前，加上：

```go
    if err := q.callAfterUpdate(q.ctx, result); err != nil {
        return nil, err
    }
```

- [ ] **Step 3: 验证编译**

Run: `go build ./...`
Expected: 编译成功

---

### Task 6: delete.go — 集成 Delete 钩子

**Files:**
- Modify: `delete.go:9-26`

- [ ] **Step 1: Delete 中调用 BeforeDelete 和 AfterDelete**

在 `delete.go:12`（`if len(q.wheres) == 0` 检查之后）之后，`buildDeleteSQL` 之前，加上：

```go
    if err := q.callBeforeDelete(q.ctx); err != nil {
        return nil, err
    }
```

在 `delete.go:24`（`return result, nil`）之前，加上：

```go
    if err := q.callAfterDelete(q.ctx, result); err != nil {
        return nil, err
    }
```

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: 编译成功

---

### Task 7: select.go — 集成 AfterQuery 钩子

**Files:**
- Modify: `select.go:162-191`

- [ ] **Step 1: scanRowsToTypeSlice 中调用 AfterQuery**

在 `select.go:185`（`results = append(results, result.Interface().(T))`）之前，加上：

```go
        data := result.Interface().(T)
        if err := q.callAfterQuery(q.ctx, &data); err != nil {
            return nil, err
        }
        results = append(results, data)
```

注意：ScannOne、ScanList 不走 scanRowsToTypeSlice，因此不触发 AfterQuery。这是合理的——这些方法用户直接控制扫描过程。

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: 编译成功

---

### Task 8: 测试

**Files:**
- Create: `sqlite_test/hooks_test.go`

- [ ] **Step 1: 编写 hooks 测试**

```go
package sqlite_test

import (
    "context"
    "errors"
    "testing"

    "github.com/shangjundragon/dbw"
    _ "github.com/glebarez/go-sqlite"
)

type HookTestEntity struct {
    ID   int64  `dbw:"primaryKey"`
    Name string
    Log  string `dbw:"dbIgnore:true"`
}

func init() {
    dbw.RegisterHooks[HookTestEntity](func(h *dbw.Hooks[HookTestEntity]) {
        h.BeforeInsert = func(ctx context.Context, data *HookTestEntity) error {
            data.Log += ":before-insert-global"
            return nil
        }
    })
}

func TestHooksGlobalBeforeInsert(t *testing.T) {
    wrapper := dbw.New[HookTestEntity](dbw.WithConfig(config))
    data := &HookTestEntity{Name: "test"}
    _, err := wrapper.Insert(data)
    if err != nil {
        t.Fatalf("insert failed: %v", err)
    }
    if data.Log != ":before-insert-global" {
        t.Errorf("global BeforeInsert hook not called, got Log=%q", data.Log)
    }
}

func TestHooksInstanceBeforeInsert(t *testing.T) {
    wrapper := dbw.New[HookTestEntity](dbw.WithConfig(config), dbw.WithHooks(func(h *dbw.Hooks[HookTestEntity]) {
        h.BeforeInsert = func(ctx context.Context, data *HookTestEntity) error {
            data.Log += ":before-insert-instance"
            return nil
        }
    }))
    data := &HookTestEntity{Name: "test2"}
    _, err := wrapper.Insert(data)
    if err != nil {
        t.Fatalf("insert failed: %v", err)
    }
    // 全局也有 BeforeInsert，所以先全局再实例
    if data.Log != ":before-insert-global:before-insert-instance" {
        t.Errorf("hooks not merged correctly, got Log=%q", data.Log)
    }
}

func TestHooksErrorPropagation(t *testing.T) {
    wantErr := errors.New("hook error")
    wrapper := dbw.New[HookTestEntity](dbw.WithConfig(config), dbw.WithHooks(func(h *dbw.Hooks[HookTestEntity]) {
        h.BeforeInsert = func(ctx context.Context, data *HookTestEntity) error {
            return wantErr
        }
    }))
    data := &HookTestEntity{Name: "test3"}
    _, err := wrapper.Insert(data)
    if !errors.Is(err, wantErr) {
        t.Errorf("expected hook error, got %v", err)
    }
}

func TestHooksAfterQuery(t *testing.T) {
    wrapper := dbw.New[HookTestEntity](dbw.WithConfig(config), dbw.WithHooks(func(h *dbw.Hooks[HookTestEntity]) {
        h.AfterQuery = func(ctx context.Context, data *HookTestEntity) error {
            data.Log = ":after-query"
            return nil
        }
    }))
    entity := &HookTestEntity{Name: "query-test"}
    _, err := wrapper.Insert(entity)
    if err != nil {
        t.Fatalf("insert failed: %v", err)
    }
    result, err := wrapper.FindOne()
    if err != nil {
        t.Fatalf("findone failed: %v", err)
    }
    if result.Log != ":after-query" {
        t.Errorf("AfterQuery hook not called, got Log=%q", result.Log)
    }
}
```

- [ ] **Step 2: 运行 hooks 测试**

Run: `go test ./sqlite_test -run TestHooks -v`
Expected: All 4 tests PASS

---

### Task 9: go vet 最终验证

- [ ] **Step 1: 全量验证**

Run: `go vet ./...`
Expected: 无警告

Run: `go build ./...`
Expected: 编译成功
