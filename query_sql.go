package dbw

import (
	"database/sql"
	"strings"
)

// buildSelectSQL builds the complete SELECT SQL statement.
func (q *DbWrapper[T]) buildSelectSQL() (string, []any) {
	if q.rawSQL != "" {
		return q.rawSQL, q.rawArgs
	}
	clone := q.cloneForLogicDel()
	var b strings.Builder
	b.WriteString("SELECT ")
	if clone.distinct {
		b.WriteString("DISTINCT ")
	}
	b.WriteString(strings.Join(clone.selects, ", "))
	b.WriteString(" FROM " + clone.getTableName())
	whereStr, whereArgs := buildWhere(clone.wheres)
	b.WriteString(whereStr)
	if len(clone.groupBy) > 0 {
		b.WriteString(" GROUP BY " + strings.Join(clone.groupBy, ", "))
	}
	if len(clone.havings) > 0 {
		b.WriteString(" HAVING ")
		for i, h := range clone.havings {
			if i > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(h.sql)
			whereArgs = append(whereArgs, h.args...)
		}
	}
	if len(clone.orders) > 0 {
		b.WriteString(" ORDER BY ")
		orders := make([]string, len(clone.orders))
		for i, o := range clone.orders {
			orders[i] = o.field + " " + o.order
		}
		b.WriteString(strings.Join(orders, ", "))
	}
	sqlStr := b.String()
	if clone.pageNum != nil && clone.pageSize != nil {
		if clone.config.PageInterceptor != nil {
			sqlStr = clone.config.PageInterceptor(sqlStr, *clone.pageNum, *clone.pageSize)
		} else {
			sqlStr = clone.config.Dialect.BuildPagination(sqlStr, *clone.pageSize, (*clone.pageNum-1)*(*clone.pageSize))
		}
	} else if clone.limit != nil || clone.offset != nil {
		limit, off := 0, 0
		if clone.limit != nil {
			limit = *clone.limit
		}
		if clone.offset != nil {
			off = *clone.offset
		}
		sqlStr = clone.config.Dialect.BuildPagination(sqlStr, limit, off)
	}
	if clone.config.Dialect.DriverName() != "mysql" && clone.config.Dialect.DriverName() != "sqlite" {
		sqlStr = clone.config.Dialect.ConvertPlaceholders(sqlStr)
	}
	return sqlStr, whereArgs
}

// query executes a SELECT query and returns the rows.
func (q *DbWrapper[T]) query() (*sql.Rows, error) {
	sqlStr, args := q.buildSelectSQL()
	debugLog(q.config, q.ctx, sqlStr, args)
	if q.tx == nil {
		return q.config.Db.QueryContext(q.ctx, sqlStr, args...)
	}
	return q.tx.QueryContext(q.ctx, sqlStr, args...)
}

// queryRow executes a SELECT query and returns a single row.
func (q *DbWrapper[T]) queryRow() *sql.Row {
	sqlStr, args := q.buildSelectSQL()
	debugLog(q.config, q.ctx, sqlStr, args)
	if q.tx == nil {
		return q.config.Db.QueryRowContext(q.ctx, sqlStr, args...)
	}
	return q.tx.QueryRowContext(q.ctx, sqlStr, args...)
}

// buildUpdateSQL builds the complete UPDATE SQL statement.
func (q *DbWrapper[T]) buildUpdateSQL(sets map[string]any) (string, []any) {
	clone := q.cloneForLogicDel()
	var b strings.Builder
	args := make([]any, 0, len(sets)+len(clone.wheres)*2)
	b.WriteString("UPDATE " + clone.getTableName() + " SET ")
	setParts := make([]string, 0, len(sets))
	for k, v := range sets {
		setParts = append(setParts, k+" = ?")
		args = append(args, v)
	}
	b.WriteString(strings.Join(setParts, ", "))
	whereStr, whereArgs := buildWhere(clone.wheres)
	b.WriteString(whereStr)
	args = append(args, whereArgs...)
	sqlStr := b.String()
	if clone.config.Dialect.DriverName() != "mysql" && clone.config.Dialect.DriverName() != "sqlite" {
		sqlStr = clone.config.Dialect.ConvertPlaceholders(sqlStr)
	}
	return sqlStr, args
}

// buildDeleteSQL builds the complete DELETE SQL statement.
func (q *DbWrapper[T]) buildDeleteSQL() (string, []any) {
	if q.meta.logicDelDbColumn != "" {
		return q.buildUpdateSQL(map[string]any{q.meta.logicDelDbColumn: q.config.LogicDeleteValue})
	}
	clone := q.cloneForLogicDel()
	var b strings.Builder
	b.WriteString("DELETE FROM " + clone.getTableName())
	whereStr, whereArgs := buildWhere(clone.wheres)
	b.WriteString(whereStr)
	sqlStr := b.String()
	if clone.config.Dialect.DriverName() != "mysql" && clone.config.Dialect.DriverName() != "sqlite" {
		sqlStr = clone.config.Dialect.ConvertPlaceholders(sqlStr)
	}
	return sqlStr, whereArgs
}
