package dbw

import "strings"

// Or marks the previous condition to connect the next one with OR instead of AND.
func (q *DbWrapper[T]) Or() *DbWrapper[T] {
	if len(q.wheres) > 0 {
		q.wheres[len(q.wheres)-1].joiner = "OR"
	}
	return q
}

// OrNest adds a parenthesized OR group condition.
func (q *DbWrapper[T]) OrNest(f func(*DbWrapper[T])) *DbWrapper[T] {
	if len(q.wheres) > 0 {
		q.wheres[len(q.wheres)-1].joiner = "OR"
	}
	qw := DbWrapper[T]{}
	f(&qw)
	str, args := buildWhere(qw.wheres)
	str = strings.ReplaceAll(str, " WHERE ", "")
	q.wheres = append(q.wheres, whereExpr{sql: "(" + str + ")", args: args, joiner: "AND"})
	return q
}

// And adds a parenthesized AND group condition.
func (q *DbWrapper[T]) And(f func(*DbWrapper[T])) *DbWrapper[T] {
	qw := DbWrapper[T]{}
	f(&qw)
	str, args := buildWhere(qw.wheres)
	str = strings.ReplaceAll(str, " WHERE ", "")
	q.wheres = append(q.wheres, whereExpr{sql: "(" + str + ")", args: args, joiner: "AND"})
	return q
}

// WhereIf adds a WHERE condition only if cond is true.
func (q *DbWrapper[T]) WhereIf(cond bool, sql string, args ...any) *DbWrapper[T] {
	if cond {
		q.Where(sql, args...)
	}
	return q
}

// AndIf adds an AND group only if cond is true.
func (q *DbWrapper[T]) AndIf(cond bool, f func(*DbWrapper[T])) *DbWrapper[T] {
	if cond {
		q.And(f)
	}
	return q
}

// OrNestIf adds an OR group only if cond is true.
func (q *DbWrapper[T]) OrNestIf(cond bool, f func(*DbWrapper[T])) *DbWrapper[T] {
	if cond {
		q.OrNest(f)
	}
	return q
}

// EqIf adds an equals condition only if cond is true.
func (q *DbWrapper[T]) EqIf(cond bool, field string, val any) *DbWrapper[T] {
	if cond {
		q.Eq(field, val)
	}
	return q
}

// LikeIf adds a LIKE condition only if cond is true.
func (q *DbWrapper[T]) LikeIf(cond bool, field, pattern string) *DbWrapper[T] {
	if cond {
		q.Like(field, pattern)
	}
	return q
}

// buildWhere builds the complete WHERE clause from a slice of whereExpr.
func buildWhere(wheres []whereExpr) (string, []any) {
	if len(wheres) == 0 {
		return "", nil
	}
	var b strings.Builder
	args := make([]any, 0, len(wheres)*2)
	b.WriteString(" WHERE ")
	for i, w := range wheres {
		if i > 0 {
			if w.joiner == "OR" {
				b.WriteString(" OR ")
			} else {
				b.WriteString(" AND ")
			}
		}
		b.WriteString(w.sql)
		args = append(args, w.args...)
	}
	return b.String(), args
}
