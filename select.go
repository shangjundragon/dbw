package dbw

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// BuildFullSelectSql 构建完整查询SQL select * from user where age > 18 order by age limit 10
func (q *DbWrapper[T]) BuildFullSelectSql() (string, []any) {
	sqlBuilder := strings.Builder{}

	// SELECT
	sqlBuilder.WriteString("SELECT ")
	if q.distinct {
		sqlBuilder.WriteString("DISTINCT ")
	}
	sqlBuilder.WriteString(strings.Join(q.selects, ", "))

	// FROM
	sqlBuilder.WriteString(" FROM " + q.getTableName())

	// 逻辑删除查询条件
	q.AndIf(q.meta.logicDelDbColumn != "", func(w *DbWrapper[T]) {
		w.Eq(q.meta.logicDelDbColumn, q.config.LogicNotDeleteValue)
	})
	// WHERE
	str, args := q.BuildWhere()
	sqlBuilder.WriteString(str)

	// GROUP BY
	if len(q.groupBy) > 0 {
		sqlBuilder.WriteString(" GROUP BY " + strings.Join(q.groupBy, ", "))
	}

	// HAVING
	if len(q.havings) > 0 {
		sqlBuilder.WriteString(" HAVING ")
		for i, h := range q.havings {
			if i > 0 {
				sqlBuilder.WriteString(" AND ")
			}
			sqlBuilder.WriteString(h.sql)
			args = append(args, h.args...)
		}
	}

	// ORDER BY
	if len(q.orders) > 0 {
		sqlBuilder.WriteString(" ORDER BY ")
		orders := make([]string, len(q.orders))
		for i, o := range q.orders {
			orders[i] = o.field + " " + o.order
		}
		sqlBuilder.WriteString(strings.Join(orders, ", "))
	}

	finalSql := ""
	if q.pageNum != nil && q.pageSize != nil {
		// 如果配置了分页拦截器
		if q.config.PageInterceptor != nil {
			finalSql = q.config.PageInterceptor(sqlBuilder.String(), *q.pageNum, *q.pageSize)
		} else { // 否则使用默认的分页方式
			offset := (*q.pageNum - 1) * (*q.pageSize)
			limit := *q.pageSize
			switch q.config.DriverName {
			case "mysql", "postgres", "sqlite":
				sqlBuilder.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset))
				finalSql = sqlBuilder.String()
			case "oracle", "sqlserver":
				sqlBuilder.WriteString(fmt.Sprintf(" OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit))
				finalSql = sqlBuilder.String()
			default:
				finalSql = sqlBuilder.String()
			}
		}

	} else {
		finalSql = sqlBuilder.String()
	}
	if q.config.PlaceholderConverter != nil {
		finalSql = q.config.PlaceholderConverter(finalSql)
	}

	return finalSql, args
}

// query 执行查询
func (q *DbWrapper[T]) query() (*sql.Rows, error) {

	sqlStr, args := q.BuildFullSelectSql()
	if q.config.Debug {
		q.PrintDebugSql(sqlStr, args)
	}

	if q.tx == nil {
		if q.config.Db == nil {
			return nil, fmt.Errorf("database connection is nil")
		}
		return q.config.Db.QueryContext(q.ctx, sqlStr, args...)
	} else {
		return q.tx.QueryContext(q.ctx, sqlStr, args...)
	}
}

// queryRow 单行查询
func (q *DbWrapper[T]) queryRow() *sql.Row {
	sqlStr, args := q.BuildFullSelectSql()
	if q.config.Debug {
		q.PrintDebugSql(sqlStr, args)
	}
	if q.tx == nil {
		return q.config.Db.QueryRowContext(q.ctx, sqlStr, args...)
	} else {
		return q.tx.QueryRowContext(q.ctx, sqlStr, args...)
	}
}

// ScanPage 查询分页 返回总条数
func (q *DbWrapper[T]) ScanPage(pageNum int, pageSize int, scanner func(*sql.Rows) error) (int64, error) {

	count, err := q.Clone().Count()
	if err != nil {
		return 0, err
	}
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 0 {
		pageSize = 0
	}
	q.pageNum = &pageNum
	q.pageSize = &pageSize
	return count, q.ScanList(scanner)
}

func (q *DbWrapper[T]) SelectPage(pageNum int, pageSize int) (records []T, count int64, err error) {
	count, err = q.Count()
	if err != nil {
		return nil, 0, err
	}

	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 0 {
		pageSize = 0
	}
	q.pageNum = &pageNum
	q.pageSize = &pageSize

	list, err := q.SelectList()
	if err != nil {
		return nil, 0, err
	}
	return list, count, err
}

// ScanOne 查询单条
func (q *DbWrapper[T]) ScanOne(dest ...any) (err error) {
	rows, err := q.query()
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		if count > 1 {
			return fmt.Errorf("expected 1 result, got %d results", count)
		}
	}
	err = rows.Scan(dest...)
	return err
}

func (q *DbWrapper[T]) SelectById(id any) (one *T, err error) {
	var t T
	if q.meta.tableIdProp == "" {
		return &t, fmt.Errorf("table id property not found")
	}
	q.Eq(q.meta.tableIdProp, id)
	return q.SelectOne()
}

func (q *DbWrapper[T]) SelectOne() (*T, error) {
	rows, err := q.query()

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	slice, err := q.scanRowsToTypeSlice(rows)
	if err != nil {
		return nil, err
	}
	if len(slice) > 1 {
		return nil, fmt.Errorf("expected 1 result, got %d result", len(slice))
	}
	if len(slice) == 0 {
		return nil, nil
	}

	return &slice[0], err

}

func (q *DbWrapper[T]) FindOne() (*T, error) {
	rows, err := q.query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	slice, err := q.scanRowsToTypeSlice(rows)
	if err != nil {
		return nil, err
	}
	if len(slice) >= 1 {
		return &slice[0], nil
	}
	return nil, fmt.Errorf("expected, got 0 result")
}

// ScanList 查询列表
func (q *DbWrapper[T]) ScanList(scanner func(*sql.Rows) error) error {
	rows, err := q.query()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scanner(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (q *DbWrapper[T]) SelectList() ([]T, error) {
	rows, err := q.query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return q.scanRowsToTypeSlice(rows)
}

// scanRowsToTypeSlice 扫描数据库行到类型切片
func (q *DbWrapper[T]) scanRowsToTypeSlice(rows *sql.Rows) ([]T, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results = make([]T, 0)
	var t T
	tType := reflect.TypeOf(t)

	// 预先验证字段映射
	for _, col := range columns {
		colLower := strings.ToLower(col)
		if _, ok := q.meta.fieldMap[colLower]; !ok {
			// 可以选择记录警告或忽略未映射的列
			if q.config.Debug {
				fmt.Printf("Warning: column %s not mapped to struct field\n", col)
			}
		}
	}

	for rows.Next() {
		// 创建T的新实例
		result := reflect.New(tType).Elem()

		// 准备扫描值
		scanValues := make([]any, len(columns))
		for i, col := range columns {
			colLower := strings.ToLower(col)
			if idx, ok := q.meta.fieldMap[colLower]; ok {
				// 获取字段地址作为扫描目标
				scanValues[i] = result.Field(idx).Addr().Interface()
			} else {
				// 如果没有匹配的字段，使用通用interface{}
				var val any
				scanValues[i] = &val
			}
		}

		// 扫描数据
		if err := rows.Scan(scanValues...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		results = append(results, result.Interface().(T))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return results, nil
}
