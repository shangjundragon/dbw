package dbw

import (
	"fmt"
	"strings"
)

// whereCondition 表示一个 WHERE 条件
type whereCondition struct {
	sql      string   // SQL 片段
	args     []any    // 参数
	isOr     bool     // 是否是 OR 条件
	nested   bool     // 是否是嵌套条件
}

// BuildWhere 构建 where 条件
func (q *DbWrapper[T]) BuildWhere() (whereStr string, args []any) {
	str := strings.Builder{}
	args = make([]any, 0)
	if len(q.wheres) > 0 {
		str.WriteString(" WHERE ")

		for i, w := range q.wheres {
			if i > 0 {
				// 根据前一个条件的 isOr 标志决定使用 AND 还是 OR
				if q.wheres[i-1].isOr {
					str.WriteString(" OR ")
				} else {
					str.WriteString(" AND ")
				}
			}
			str.WriteString(w.sql)
			args = append(args, w.args...)
		}
	}
	return str.String(), args
}

func (q *DbWrapper[T]) Or() *DbWrapper[T] {
	if q.whereIsOrIndexes == nil {
		q.whereIsOrIndexes = make(map[int]any)
	}
	q.whereIsOrIndexes[len(q.wheres)] = true
	return q
}

func (q *DbWrapper[T]) OrNest(f func(*DbWrapper[T])) *DbWrapper[T] {
	// 先标记需要 OR
	if len(q.wheres) > 0 {
		q.wheres[len(q.wheres)-1].isOr = true
	}
	qw := DbWrapper[T]{}
	f(&qw)
	str, args := qw.BuildWhere()
	str = strings.ReplaceAll(str, "WHERE", "")
	sqlStr := fmt.Sprintf("(%s )", str)
	q.wheres = append(q.wheres, whereExpr{sql: sqlStr, args: args})
	return q
}

func (q *DbWrapper[T]) OrNestIf(c bool, f func(*DbWrapper[T])) *DbWrapper[T] {
	if !c {
		return q
	}
	return q.OrNest(f)
}

func (q *DbWrapper[T]) And(w func(*DbWrapper[T])) *DbWrapper[T] {
	qw := DbWrapper[T]{}
	w(&qw)
	str, args := qw.BuildWhere()
	str = strings.ReplaceAll(str, "WHERE", "")
	sqlStr := fmt.Sprintf("(%s )", str)
	q.wheres = append(q.wheres, whereExpr{sql: sqlStr, args: args})
	return q
}

// Where WHERE 条件
func (q *DbWrapper[T]) Where(sql string, args ...any) *DbWrapper[T] {
	q.wheres = append(q.wheres, whereExpr{sql: sql, args: args})
	return q
}

// WhereIf 条件判断
func (q *DbWrapper[T]) WhereIf(cond bool, sql string, args ...any) *DbWrapper[T] {
	if cond {
		q.Where(sql, args...)
	}
	return q
}

// AndIf 条件判断 AND
func (q *DbWrapper[T]) AndIf(c bool, w func(*DbWrapper[T])) *DbWrapper[T] {
	if !c {
		return q
	}
	return q.And(w)
}

// Eq 等于
func (q *DbWrapper[T]) Eq(field string, val any) *DbWrapper[T] {
	return q.Where(field+" = ?", val)
}

func (q *DbWrapper[T]) EqIf(c bool, field string, val any) *DbWrapper[T] {
	if !c {
		return q
	}
	return q.Where(field+" = ?", val)
}

// Ne 不等于
func (q *DbWrapper[T]) Ne(field string, val any) *DbWrapper[T] {
	return q.Where(field+" != ?", val)
}

// Gt 大于
func (q *DbWrapper[T]) Gt(field string, val any) *DbWrapper[T] {
	return q.Where(field+" > ?", val)
}

// Ge 大于等于
func (q *DbWrapper[T]) Ge(field string, val any) *DbWrapper[T] {
	return q.Where(field+" >= ?", val)
}

// Lt 小于
func (q *DbWrapper[T]) Lt(field string, val any) *DbWrapper[T] {
	return q.Where(field+" < ?", val)
}

// Le 小于等于
func (q *DbWrapper[T]) Le(field string, val any) *DbWrapper[T] {
	return q.Where(field+" <= ?", val)
}

// Like LIKE ex field=username pattern=a%
func (q *DbWrapper[T]) Like(field, pattern string) *DbWrapper[T] {
	return q.Where(field+" LIKE ?", pattern)
}

func (q *DbWrapper[T]) LikeIf(c bool, field, pattern string) *DbWrapper[T] {
	if !c {
		return q
	}
	return q.Where(field+" LIKE ?", pattern)
}

// In IN
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

func (q *DbWrapper[T]) Between(field string, min any, max any) *DbWrapper[T] {
	return q.Where(field+" BETWEEN ? AND ?", min, max)
}

// IsNull IS NULL
func (q *DbWrapper[T]) IsNull(field string) *DbWrapper[T] {
	return q.Where(field + " IS NULL")
}

// NotNull IS NOT NULL
func (q *DbWrapper[T]) NotNull(field string) *DbWrapper[T] {
	return q.Where(field + " IS NOT NULL")
}
