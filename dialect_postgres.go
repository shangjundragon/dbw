package dbw

import (
	"fmt"
	"strings"
)

// postgresDialect is the PostgreSQL database dialect implementation.
type postgresDialect struct{}

func (d *postgresDialect) DriverName() string  { return "postgres" }
func (d *postgresDialect) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }
func (d *postgresDialect) ConvertPlaceholders(sql string) string {
	var buf strings.Builder
	paramIndex := 1
	for i := 0; i < len(sql); i++ {
		if sql[i] == '?' {
			fmt.Fprintf(&buf, "$%d", paramIndex)
			paramIndex++
		} else {
			buf.WriteByte(sql[i])
		}
	}
	return buf.String()
}
func (d *postgresDialect) QuoteIdentifier(name string) string { return `"` + name + `"` }
func (d *postgresDialect) BuildPagination(sql string, limit, offset int) string {
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, limit, offset)
}
