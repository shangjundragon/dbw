package dbw

import "fmt"

// mysqlDialect is the MySQL database dialect implementation.
type mysqlDialect struct{}

func (d *mysqlDialect) DriverName() string                { return "mysql" }
func (d *mysqlDialect) Placeholder(n int) string          { return "?" }
func (d *mysqlDialect) ConvertPlaceholders(sql string) string { return sql }
func (d *mysqlDialect) QuoteIdentifier(name string) string { return "`" + name + "`" }
func (d *mysqlDialect) BuildPagination(sql string, limit, offset int) string {
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, limit, offset)
}
