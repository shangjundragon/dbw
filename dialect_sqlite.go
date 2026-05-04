package dbw

import "fmt"

// sqliteDialect is the SQLite database dialect implementation.
type sqliteDialect struct{}

func (d *sqliteDialect) DriverName() string                { return "sqlite" }
func (d *sqliteDialect) Placeholder(n int) string          { return "?" }
func (d *sqliteDialect) ConvertPlaceholders(sql string) string { return sql }
func (d *sqliteDialect) QuoteIdentifier(name string) string { return `"` + name + `"` }
func (d *sqliteDialect) BuildPagination(sql string, limit, offset int) string {
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, limit, offset)
}
