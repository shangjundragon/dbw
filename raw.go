package dbw

import (
	"database/sql"
	"fmt"
)

// Raw sets a raw SQL query with placeholders.
func (q *DbWrapper[T]) Raw(sql string, args ...any) *DbWrapper[T] {
	q.rawSQL = sql
	q.rawArgs = args
	return q
}

// Exec executes the raw SQL statement.
func (q *DbWrapper[T]) Exec() (sql.Result, error) {
	sqlStr := q.rawSQL
	args := q.rawArgs
	if sqlStr == "" {
		return nil, fmt.Errorf("dbw: raw SQL is required for Exec(), use Raw() first")
	}
	if q.config.Dialect.DriverName() != "mysql" && q.config.Dialect.DriverName() != "sqlite" {
		sqlStr = q.config.Dialect.ConvertPlaceholders(sqlStr)
	}
	debugLog(q.config, q.ctx, sqlStr, args)
	if q.tx == nil {
		return q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	}
	return q.tx.ExecContext(q.ctx, sqlStr, args...)
}
