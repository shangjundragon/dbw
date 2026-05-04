package dbw

// Select sets the columns to select.
func (q *DbWrapper[T]) Select(fields ...string) *DbWrapper[T] {
	if len(fields) > 0 {
		q.selects = fields
	}
	return q
}

// OrderBy adds an ascending ORDER BY clause.
func (q *DbWrapper[T]) OrderBy(field string) *DbWrapper[T] {
	q.orders = append(q.orders, orderExpr{field: field, order: "ASC"})
	return q
}

// OrderByDesc adds a descending ORDER BY clause.
func (q *DbWrapper[T]) OrderByDesc(field string) *DbWrapper[T] {
	q.orders = append(q.orders, orderExpr{field: field, order: "DESC"})
	return q
}

// GroupBy adds GROUP BY columns.
func (q *DbWrapper[T]) GroupBy(fields ...string) *DbWrapper[T] {
	q.groupBy = append(q.groupBy, fields...)
	return q
}

// Having adds a HAVING condition.
func (q *DbWrapper[T]) Having(sql string, args ...any) *DbWrapper[T] {
	q.havings = append(q.havings, whereExpr{sql: sql, args: args})
	return q
}

// Distinct enables DISTINCT in the SELECT clause.
func (q *DbWrapper[T]) Distinct() *DbWrapper[T] {
	q.distinct = true
	return q
}

// Limit sets the maximum number of rows to return.
func (q *DbWrapper[T]) Limit(n int) *DbWrapper[T] {
	q.limit = &n
	return q
}

// Offset sets the number of rows to skip.
func (q *DbWrapper[T]) Offset(n int) *DbWrapper[T] {
	q.offset = &n
	return q
}

// Count returns the total number of matching rows.
func (q *DbWrapper[T]) Count() (int64, error) {
	qCopy := q.Clone()
	qCopy.selects = []string{"COUNT(*)"}
	qCopy.orders = nil
	qCopy.limit = nil
	qCopy.offset = nil
	qCopy.pageNum = nil
	qCopy.pageSize = nil
	var count int64
	err := qCopy.queryRow().Scan(&count)
	return count, err
}

// Exist returns whether any matching rows exist.
func (q *DbWrapper[T]) Exist() (bool, error) {
	count, err := q.Count()
	return count > 0, err
}
