# DBW Hooks — 生命周期钩子设计文档

## 概述

为 DBW 添加通用生命周期钩子（Hook）机制，在执行 Insert、Update、Delete、Select 操作时提供回调点，使用者可注入自定义逻辑（如写入修改者/更新者、修改 SQL、审计日志等）。

## 设计原则

1. **类型安全** — 通过 Go 1.21 泛型保证每个 `Hooks[T]` 绑定到特定实体类型
2. **全局 + 实例双层** — 全局注册基线行为，实例级覆盖/扩展
3. **可选 nil** — 所有钩子字段均为 nilable，nil 跳过
4. **错误传播** — 钩子返回 error 则中断当前操作
5. **与现有模式一致** — `WithHooks` 采用函数选项模式，与 `WithConfig`/`WithTx` 相同

## Hook 点定义

```go
// hooks.go
package dbw

type Hooks[T any] struct {
    // Insert
    BeforeInsert    func(ctx context.Context, data *T) error
    AfterInsert     func(ctx context.Context, data *T, result sql.Result) error

    // UpdateById
    BeforeUpdate    func(ctx context.Context, data *T) error

    // Update(values map)
    BeforeUpdateMap func(ctx context.Context, values map[string]any) error

    // Update (both)
    AfterUpdate     func(ctx context.Context, result sql.Result) error

    // Delete / DeleteById / DeleteByIds
    BeforeDelete    func(ctx context.Context) error
    AfterDelete     func(ctx context.Context, result sql.Result) error

    // Select (per-row, after scanning)
    AfterQuery      func(ctx context.Context, data *T) error
}
```

所有字段均为 `func` 类型，nil 表示不启用。

## 全局注册

```go
// 全局注册（进程级，线程安全）
func RegisterHooks[T any](fn func(h *Hooks[T]))
```

实现：

```go
func RegisterHooks[T any](fn func(h *Hooks[T])) {
    key := reflect.TypeFor[T]()
    raw, _ := globalHooks.LoadOrStore(key, &Hooks[T]{})
    fn(raw.(*Hooks[T]))  // 合并到已有 hooks 上
}
```

- 全局 `sync.Map` 存储，key 为 `reflect.Type`，value 为 `*Hooks[T]`
- 多次调用 `RegisterHooks` 会**合并**：`fn` 收到已有的 `*Hooks[T]`，覆盖特定字段，其余字段保留
- 使用 `LoadOrStore` 保证并发安全，首次创建空 `*Hooks[T]`
- 任意类型 `T` 均可独立注册

## 实例级注入

```go
// WithHooks 函数选项
func WithHooks[T any](fn func(h *Hooks[T])) Options
```

```go
// 实例内部存储
type DbWrapper[T any] struct {
    hooks any         // 实际为 *Hooks[T]，因 Options 签名 func(*DbWrapper[any]) 限制存为 any
    // ... 其余字段不变
}
```

`WithHooks` 实现：

```go
func WithHooks[T any](fn func(h *Hooks[T])) Options {
    h := &Hooks[T]{}
    fn(h)
    return func(opts *DbWrapper[any]) {
        opts.hooks = h   // *Hooks[T] 存为 any
    }
}
```

使用处做类型断言：`q.hooks.(*Hooks[T])`（安全，因为 `New[T]` 时注入的必然是 `*Hooks[T]`）。

## 合并策略

执行时按以下优先级：

1. **全局钩子** — 从 `sync.Map` 取出，如果非 nil 且对应 hook point 非 nil，先执行
2. **实例钩子** — `DbWrapper.hooks` 如果非 nil 且对应 hook point 非 nil，后执行

合并是在执行时动态完成的（lazy merge），不创建新 `Hooks[T]` 对象。

## 执行时机（集成点）

### Insert（insert.go）

```
Insert(data *T):
  1. 校验 data != nil
  2. beforeInsert(data)         ← 现有：填充主键/时间戳
  3. BeforeInsert(ctx, data)    ← 新：钩子回调
  4. 构建 SQL
  5. ExecContext
  6. AfterInsert(ctx, data, r)  ← 新：钩子回调
  7. 返回 result
```

### UpdateById（update.go）

```
UpdateById(data *T):
  1. 校验主键
  2. BeforeUpdate(ctx, data)    ← 新：钩子回调
  3. 遍历字段，构建 SET
  4. 构建 SQL
  5. ExecContext
  6. AfterUpdate(ctx, r)        ← 新：钩子回调
  7. 返回 result
```

### Update(values map)（update.go）

```
Update(values map):
  1. 校验
  2. BeforeUpdateMap(ctx, map)  ← 新：钩子回调，可修改 map
  3. 追加 autoUpdateTime
  4. 构建 SQL
  5. ExecContext
  6. AfterUpdate(ctx, r)        ← 新：钩子回调
  7. 返回 result
```

### Delete（delete.go）

```
Delete():
  1. 校验 WHERE
  2. BeforeDelete(ctx)          ← 新：钩子回调
  3. 构建 SQL（逻辑删除或物理删除）
  4. ExecContext
  5. AfterDelete(ctx, r)        ← 新：钩子回调
  6. 返回 result
```

### Select（select.go 中的 scanRowsToTypeSlice）

```
scanRowsToTypeSlice(rows):
  for rows.Next():
    scan row → T
    AfterQuery(ctx, &data)      ← 新：每行钩子回调
    append to slice
  return slice, nil
```

- `SelectOne` / `FindOne` 也走 `scanRowsToTypeSlice`，因此同样适用
- `Count` / `Exist` 不走 scanRows，不触发 AfterQuery

## 内部实现模式

每个 hook point 使用独立的 `callXxx` 辅助方法，内部按**全局 → 实例**顺序执行：

```go
func (q *DbWrapper[T]) callBeforeInsert(ctx context.Context, data *T) error {
    if raw, ok := globalHooks.Load(reflect.TypeFor[T]()); ok {
        if h := raw.(*Hooks[T]); h.BeforeInsert != nil {
            if err := h.BeforeInsert(ctx, data); err != nil {
                return err
            }
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

共需 8 个辅助方法：`callBeforeInsert`、`callAfterInsert`、`callBeforeUpdate`、`callBeforeUpdateMap`、`callAfterUpdate`、`callBeforeDelete`、`callAfterDelete`、`callAfterQuery`。

全局注册存储：

```go
var globalHooks sync.Map  // key = reflect.Type, value = *Hooks[T]
```

## Clone / Clean / Reset 行为

| 方法 | hooks 字段处理 |
|------|---------------|
| `Clone()` | 浅拷贝 `*Hooks[T]` 指针（安全，hooks 不变） |
| `Clean()` | 保留 `hooks` 指针 |
| `Reset()` | 重置 `hooks` 为 nil（因为是新实例） |

## 错误处理

- 任意钩子返回 error → 当前操作立即中止，error 原样返回给调用者
- `BeforeInsert` / `BeforeUpdate` 返回 error → SQL 不执行
- `AfterInsert` / `AfterUpdate` 等后置钩子返回 error → SQL 已执行，无法回滚（除非在事务中由调用者处理）
- non-error panic 不捕获

## 线程安全

- `RegisterHooks[T]`：全局 `sync.Map` 线程安全
- `WithHooks`：只在 `New[T]` 时设置一次，后续 `hooks` 不修改
- 全局 hooks 在 `DbWrapper` 方法中被只读读取

## 测试策略

- **单元测试**：新建 `hooks_test.go`，测试：
  - 全局注册 + 执行
  - 实例级覆盖 + 执行
  - 全局+实例合并执行顺序
  - error 传播
  - hooks nil 时不影响正常操作
- **集成测试**：在 `sqlite_test` 和 `mysql_test` 中增加钩子场景

## 文件变更清单

| 文件 | 操作 | 变更内容 |
|------|------|---------|
| `hooks.go` | **新建** | `Hooks[T]` 定义、`RegisterHooks[T]`、`getGlobalHooks[T]`、`WithHooks` |
| `wrapper.go` | 修改 | `DbWrapper[T]` 加 `hooks any` 字段；`Clone`/`Clean`/`Reset` 传播 |
| `insert.go` | 修改 | `Insert` 中触发 BeforeInsert / AfterInsert |
| `update.go` | 修改 | `UpdateById` 中触发 BeforeUpdate / AfterUpdate；`Update` 中触发 BeforeUpdateMap / AfterUpdate |
| `delete.go` | 修改 | `Delete` 中触发 BeforeDelete / AfterDelete |
| `select.go` | 修改 | `scanRowsToTypeSlice` 中触发 AfterQuery |
