# AGENTS.md — DBW

Go 1.21 ORM wrapper (`github.com/shangjundragon/dbw`). Single module, no build system beyond `go`.

## Testing

- **SQLite (self-contained):** `go test ./sqlite_test`
  - Uses `:memory:?cache=shared`, creates tables in `init()`, safe to run anywhere.
- **MySQL (needs real DB):** `go test ./mysql_test`
  - Hardcoded DSN in `mysql_test/mysql_test.go:24` points to `192.168.31.52:3306`.
  - Will fail if no MySQL reachable. Skip or edit DSN before running.
- No other test targets. No `go test ./...` coverage because MySQL tests will crash.

## Architecture Notes

- **Entrypoint:** `dbw.New[T]()` in `dbwrapper.go`.
- **Config:** `NewConfig()` requires `Db`, `DriverName` (`mysql|sqlite|postgres|oracle|sqlserver`).
- **Tags:** `dbw:"primaryKey"`, `dbw:"tableLogic"`, `dbw:"autoCreateTime:milli"`, `dbw:"autoUpdateTime"`, `dbw:"default:x"`, `dbw:"dbIgnore:true"`, `dbw:"column:name"`, `dbw:"tableUpdateStrategy:always"`.
- **Table name:** Auto-converts PascalCase → snake_case via `getTableName()`. Override with `Tabler` interface (`TableName() string`).
- **ID gen:** Default `snowflake`. Register custom with `RegisterIdGenerator("key", fn)`.
- **Safety:** Update/delete without WHERE returns error (except `UpdateById`).
- **Placeholder:** MySQL `?` by default. Set `config.PlaceholderConverter = dbw.PostgreSConverter` for Postgres `$N`.

## Quirks / Gotchas

- `InitConfig()` sets a global default config. `NewConfig()` creates an instance config. Prefer passing config explicitly (`dbw.WithConfig`) to avoid global state issues in tests.
- `dbw.WithTx(tx)` and `dbw.WithConfig(cfg)` are separate functional options; both can be passed to `New[T](...)`. In MySQL tests, `Tx(tx)` is also used as shorthand for `WithTx(tx)`.
- Snowflake init in `common.go` is lazy via `GetSnowflake()`. No external dependencies for it.
- SQLite dependency is `github.com/glebarez/go-sqlite` (CGO-free), not `mattn/go-sqlite3`.
- `go.mod` has no test-only deps; everything is in the main module.

## Commands

```bash
go test ./sqlite_test          # safe, fast
go test ./sqlite_test -bench=.  # includes insert/select benchmarks
go vet ./...
go build ./...
```

No linter config, no CI, no code generator.
