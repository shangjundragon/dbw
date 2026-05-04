# AGENTS.md — DBW

Go 1.21 ORM wrapper (`github.com/shangjundragon/dbw`). Single module, no build system beyond `go`.

## Testing

- **SQLite (self-contained):** `go test ./sqlite_test`
  - Uses `:memory:?cache=shared`, creates tables in `init()`, safe to run anywhere.
  - Benchmarks: `go test ./sqlite_test -bench=.`
- **MySQL (needs real DB):** `go test ./mysql_test`
  - Hardcoded DSN in `mysql_test/mysql_test.go:24` points to `192.168.31.52:3306`.
  - Will fail if no MySQL reachable. Skip or edit DSN before running.
- No `go test ./...` — MySQL tests will crash without a reachable DB.

## Architecture Notes

- **Entrypoint:** `dbw.New[T](dbw.WithConfig(config))` in `wrapper.go`.
- **Config:** `dbw.NewConfig(func(config *Config) { config.Db = db; ... })` — created via callback, not struct literal. `Db` is required (panics if nil).
- **Dialect:** Interface in `config.go`, implementations in `dialect_*.go`. Auto-selected by `config.DriverName` (`mysql|sqlite|postgres|oracle`). Replaces old `PlaceholderConverter`.
- **Tags** (separator is `;`, e.g. `dbw:"primaryKey;idGenerator:custom"`):
  - `primaryKey` — marks ID field. Auto-generates via `snowflake` by default or `idGenerator` value.
  - `idGenerator:name` — use a registered generator (e.g. `dbw:"primaryKey;idGenerator:uuid"`).
  - `tableLogic` — logical delete flag.
  - `autoCreateTime` / `autoUpdateTime` — `"autoCreateTime"` → `time.Time`; `"autoCreateTime:milli"` → `int64` unix millis.
  - `default:x` — default value when field is zero.
  - `dbIgnore:true` — skip field entirely (column NOT in DB).
  - `column:name` — override DB column name (now works on primaryKey too).
  - `tableUpdateStrategy:always` — force include in UPDATE SET even when zero/nil.
- **Table name:** Auto PascalCase → snake_case via `getTableName()` in `table.go`. Override with `Tabler` interface (`TableName() string`). Override at instance level with `.TableName("t")`.
- **ID gen:** Default `snowflake` (int64/uint64/int) or `snowflakeStr` (string). Register custom: `dbw.RegisterIdGenerator("key", func() any { ... })`. Defined in `snowflake.go` + `generator.go`.
- **Safety:** `Update(values)` blocks without WHERE. `Delete()` blocks without WHERE. `UpdateById` gets through. Returns structured error `ErrNoWhereClause`.
- **Structured errors:** `errors.go` — `ErrRecordNotFound`, `ErrMultipleRecords`, `ErrNoWhereClause`, `ErrNoFieldsToUpdate`, `ErrNoPrimaryKey`, `ErrBatchTooLarge`, `ErrEmptyData`, `ErrNilEntity`. All support `errors.Is()`.
- **Logging:** `config.Debug = true` for built-in printf logging. Use `dbw.SetLogFn(fn)` for custom logger. Defined in `log.go`.

## Quirks / Gotchas

- `dbw.WithTx(tx)` and `dbw.WithConfig(cfg)` are functional options for `New[T](...)`. `DbWrapper.Tx(tx)` is an instance method (does the same).
- Snowflake is lazily initialized in `GetSnowflake()` (`snowflake.go`). Double-checked locking, thread-safe. Machine ID uses `atomic.Int64`.
- `Or()` sets the **last** `whereExpr.joiner` to `"OR"`, so the **next** `Where()`/`Eq()` etc. connects with OR. Fixed from v0 (was broken).
- `InsertBatch` hard-limits to **1000 records**. Use `InsertBatchSplit(data, size)` for larger batches.
- `InsertBatch` uses `Field(index)` O(1) access, not `FieldByName` (fixed from v0).
- `SelectById` → uses `SelectOne` → returns error if >1 rows match. Now uses `tableIdDbColumn` (not `tableIdFiledName`) — fixed from v0.
- `FindOne` → returns **first** row even if multiple match. 0 rows returns `ErrRecordNotFound`.
- `SelectOne` → 0 rows returns `(nil, nil)`. >1 rows returns `ErrMultipleRecords`. Uses `LIMIT 2` to avoid full scan.
- `SelectPage` defaults `pageSize` to 10 if < 1; returns empty slice (not nil) when count=0.
- SQL queries auto-append `AND logic_del_column = '0'` when `tableLogic` tag exists (via `cloneForLogicDel()`).
- `Delete()` with `tableLogic` field performs an UPDATE, not a real DELETE.
- `And(fn)` / `OrNest(fn)` for parenthesized groups. `buildWhere()` is the internal function in `condition_group.go`.
- New features: `Raw()` + `Exec()` / `SelectList()` for raw SQL. `WhereStruct()` for struct-based conditions. `Limit()` / `Offset()` independent of pagination.
- `camelToSnake()` now uses same abbreviation-aware algorithm as `getTableName()` (HTTPServer→http_server, not h_t_t_p_server).
- SQLite driver: `github.com/glebarez/go-sqlite` (CGO-free). Import side-effect: `_ "github.com/glebarez/go-sqlite"`.
- `go.mod` has no test-only deps; everything is in the main module.

## File Map

| File | Purpose |
|------|---------|
| `wrapper.go` | `DbWrapper[T]`, `New`, `Options`, `ExecuteTx`, `Clone`, `Clean`, `Reset` |
| `config.go` | `Config`, `Dialect` interface, `NewConfig` |
| `dialect_*.go` | MySQL/SQLite/PostgreSQL/Oracle dialect implementations |
| `condition.go` | `Where`, `Eq`, `Ne`, `Gt`, `Ge`, `Lt`, `Le`, `Like`, `In`, `Between`, `IsNull`, `NotNull` |
| `condition_group.go` | `Or`, `And`, `OrNest`, `WhereIf`, `AndIf`, `EqIf`, `LikeIf`, `OrNestIf`, `buildWhere` |
| `query.go` | `Select`, `OrderBy`, `GroupBy`, `Having`, `Distinct`, `Limit`, `Offset`, `Count`, `Exist` |
| `query_sql.go` | `buildSelectSQL`, `buildUpdateSQL`, `buildDeleteSQL`, `query`, `queryRow` (internal) |
| `select.go` | `SelectOne`, `FindOne`, `SelectList`, `SelectById`, `SelectPage`, `Scan*`, `scanRowsToTypeSlice` |
| `insert.go` | `Insert`, `InsertBatch`, `InsertBatchSplit`, `beforeInsert` |
| `update.go` | `UpdateById`, `Update` |
| `delete.go` | `Delete`, `DeleteById`, `DeleteByIds` |
| `raw.go` | `Raw`, `Exec` |
| `where_struct.go` | `WhereStruct` |
| `meta.go` | `structMeta`, `fieldInfo`, `getStructMeta`, `resolveDbwTag`, `setIdMeta` |
| `table.go` | `getTableName`, `camelToSnake`, `Tabler` |
| `snowflake.go` | `Snowflake`, `GetSnowflake`, `SetSnowflakeMachineId` |
| `generator.go` | `RegisterIdGenerator`, default generators |
| `errors.go` | Structured error variables |
| `log.go` | `SetLogFn`, `debugLog` |
| `convert.go` | `convertDefaultValue`, `getTime` |
| `utils.go` | `GetInt64Ptr`, `GetStringPtr`, `sliceSplit`, `GetContextWithTimeout` |

## Commands

```bash
go test ./sqlite_test              # safe, fast
go test ./sqlite_test -bench=.     # includes insert/select benchmarks
go vet ./...
go build ./...
```

No linter config, no CI, no code generator.
