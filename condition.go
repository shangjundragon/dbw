package dbw

import "strings"

// Where adds a raw WHERE condition with placeholders.
func (q *DbWrapper[T]) Where(sql string, args ...any) *DbWrapper[T] {
	joiner := "AND"
	if len(q.wheres) > 0 && q.wheres[len(q.wheres)-1].joiner == "OR" {
		joiner = "OR"
	}
	q.wheres = append(q.wheres, whereExpr{sql: sql, args: args, joiner: joiner})
	return q
}

// Eq adds an equals condition (field = value).
func (q *DbWrapper[T]) Eq(field string, val any) *DbWrapper[T] {
	return q.Where(field+" = ?", val)
}

// Ne adds a not-equals condition (field != value).
func (q *DbWrapper[T]) Ne(field string, val any) *DbWrapper[T] {
	return q.Where(field+" != ?", val)
}

// Gt adds a greater-than condition (field > value).
func (q *DbWrapper[T]) Gt(field string, val any) *DbWrapper[T] {
	return q.Where(field+" > ?", val)
}

// Ge adds a greater-than-or-equal condition (field >= value).
func (q *DbWrapper[T]) Ge(field string, val any) *DbWrapper[T] {
	return q.Where(field+" >= ?", val)
}

// Lt adds a less-than condition (field < value).
func (q *DbWrapper[T]) Lt(field string, val any) *DbWrapper[T] {
	return q.Where(field+" < ?", val)
}

// Le adds a less-than-or-equal condition (field <= value).
func (q *DbWrapper[T]) Le(field string, val any) *DbWrapper[T] {
	return q.Where(field+" <= ?", val)
}

// Like adds a LIKE condition.
func (q *DbWrapper[T]) Like(field, pattern string) *DbWrapper[T] {
	return q.Where(field+" LIKE ?", pattern)
}

// In adds an IN condition.
func (q *DbWrapper[T]) In(field string, values ...any) *DbWrapper[T] {
	if len(values) == 0 {
		return q.Where("1 = 0")
	}
	placeholders := make([]string, len(values))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return q.Where(field+" IN ("+strings.Join(placeholders, ",")+")", values...)
}

// Between adds a BETWEEN condition.
func (q *DbWrapper[T]) Between(field string, min any, max any) *DbWrapper[T] {
	return q.Where(field+" BETWEEN ? AND ?", min, max)
}

// IsNull adds an IS NULL condition.
func (q *DbWrapper[T]) IsNull(field string) *DbWrapper[T] {
	return q.Where(field + " IS NULL")
}

// NotNull adds an IS NOT NULL condition.
func (q *DbWrapper[T]) NotNull(field string) *DbWrapper[T] {
	return q.Where(field + " IS NOT NULL")
}
