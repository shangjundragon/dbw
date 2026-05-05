package dbw

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// SelectById retrieves a single record by its primary key.
func (q *DbWrapper[T]) SelectById(id any) (*T, error) {
	if q.meta.tableIdFieldName == "" {
		return nil, ErrNoPrimaryKey
	}
	return q.Clone().Eq(q.meta.tableIdDbColumn, id).SelectOne()
}

// SelectByIds retrieves multiple records by their primary keys.
func (q *DbWrapper[T]) SelectByIds(ids []any) ([]T, error) {
	if len(ids) == 0 {
		return make([]T, 0), nil
	}
	if q.meta.tableIdFieldName == "" {
		return nil, ErrNoPrimaryKey
	}
	return q.Clone().In(q.meta.tableIdDbColumn, ids...).SelectList()
}

// SelectOne retrieves a single record, returning error if multiple match.
func (q *DbWrapper[T]) SelectOne() (*T, error) {
	clone := q.Clone()
	clone.Limit(2)
	rows, err := clone.query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	slice, err := clone.scanRowsToTypeSlice(rows)
	if err != nil {
		return nil, err
	}
	if len(slice) > 1 {
		return nil, fmt.Errorf("%w: got %d", ErrMultipleRecords, len(slice))
	}
	if len(slice) == 0 {
		return nil, nil
	}
	return &slice[0], nil
}

// FindOne retrieves the first matching record.
func (q *DbWrapper[T]) FindOne() (*T, error) {
	clone := q.Clone()
	clone.Limit(1)
	rows, err := clone.query()
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
	return nil, ErrRecordNotFound
}

// SelectList retrieves all matching records.
func (q *DbWrapper[T]) SelectList() ([]T, error) {
	rows, err := q.query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return q.scanRowsToTypeSlice(rows)
}

// SelectPage retrieves paginated records with total count.
func (q *DbWrapper[T]) SelectPage(pageNum int, pageSize int) (records []T, count int64, err error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	count, err = q.Clone().Count()
	if err != nil {
		return nil, 0, fmt.Errorf("count failed: %w", err)
	}
	if count == 0 {
		return make([]T, 0), 0, nil
	}
	clone := q.Clone()
	clone.pageNum = &pageNum
	clone.pageSize = &pageSize
	list, err := clone.SelectList()
	if err != nil {
		return nil, 0, fmt.Errorf("select list failed: %w", err)
	}
	return list, count, nil
}

// ScanOne scans a single row into the given destinations.
func (q *DbWrapper[T]) ScanOne(dest ...any) error {
	clone := q.Clone()
	clone.Limit(2)
	rows, err := clone.query()
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		if count > 1 {
			return fmt.Errorf("%w: got %d", ErrMultipleRecords, count)
		}
		if err := rows.Scan(dest...); err != nil {
			return err
		}
	}
	if count == 0 {
		return nil
	}
	return rows.Err()
}

// ScanList scans all rows using the provided scanner function.
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

// ScanPage scans paginated rows using the provided scanner function.
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

func (q *DbWrapper[T]) scanRowsToTypeSlice(rows *sql.Rows) ([]T, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	results := make([]T, 0)
	var t T
	tType := reflect.TypeOf(t)
	for rows.Next() {
		result := reflect.New(tType).Elem()
		scanValues := make([]any, len(columns))
		for i, col := range columns {
			colLower := strings.ToLower(col)
			if idx, ok := q.meta.fieldMap[colLower]; ok {
				scanValues[i] = result.Field(idx).Addr().Interface()
			} else {
				var val any
				scanValues[i] = &val
			}
		}
		if err := rows.Scan(scanValues...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		data := result.Interface().(T)
		if err := q.callAfterQuery(&data); err != nil {
			return nil, err
		}
		results = append(results, data)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}
	return results, nil
}
