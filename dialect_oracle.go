package dbw

import (
	"fmt"
	"strings"
)

// oracleDialect is the Oracle database dialect implementation.
type oracleDialect struct{}

func (d *oracleDialect) DriverName() string  { return "oracle" }
func (d *oracleDialect) Placeholder(n int) string { return fmt.Sprintf(":%d", n) }
func (d *oracleDialect) ConvertPlaceholders(sql string) string {
	var buf strings.Builder
	paramIndex := 1
	for i := 0; i < len(sql); i++ {
		if sql[i] == '?' {
			fmt.Fprintf(&buf, ":%d", paramIndex)
			paramIndex++
		} else {
			buf.WriteByte(sql[i])
		}
	}
	return buf.String()
}
func (d *oracleDialect) QuoteIdentifier(name string) string { return `"` + name + `"` }
func (d *oracleDialect) BuildPagination(sql string, limit, offset int) string {
	return fmt.Sprintf("%s OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", sql, offset, limit)
}
