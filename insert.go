package dbw

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

func (q *DbWrapper[T]) beforeInsert(data *T) (generateTableId any, err error) {
	valueOf := reflect.ValueOf(data).Elem()
	for _, fieldInfo := range q.meta.fieldsInfoMap {
		fieldValue := valueOf.Field(fieldInfo.index)
		if fieldInfo.dbColumn == q.meta.tableIdDbColumn {
			if fieldValue.IsZero() {
				fn, ok := idGenerator[q.meta.idGenerator]
				if !ok {
					return nil, fmt.Errorf("dbw: id generator %s not found", q.meta.idGenerator)
				}
				generateTableId = fn()
				fieldValue.Set(reflect.ValueOf(generateTableId))
			}
			continue
		}
		if autoCreateTime := fieldInfo.dbwTag["autoCreateTime"]; autoCreateTime != "" {
			fieldValue.Set(reflect.ValueOf(getTime(autoCreateTime)))
			continue
		}
		if autoUpdateTime := fieldInfo.dbwTag["autoUpdateTime"]; autoUpdateTime != "" {
			fieldValue.Set(reflect.ValueOf(getTime(autoUpdateTime)))
			continue
		}
		if q.meta.logicDelDbColumn != "" && fieldInfo.dbColumn == q.meta.logicDelDbColumn {
			fieldValue.Set(reflect.ValueOf(q.config.LogicNotDeleteValue))
			continue
		}
		if fieldValue.IsZero() {
			if defaultValue, has := fieldInfo.dbwTag["default"]; has {
				convertedVal := convertDefaultValue(defaultValue, fieldValue.Type())
				fieldValue.Set(reflect.ValueOf(convertedVal))
			}
		}
	}
	return generateTableId, nil
}

// Insert inserts a single record and returns the result.
func (q *DbWrapper[T]) Insert(data *T) (sql.Result, error) {
	if data == nil {
		return nil, ErrNilEntity
	}
	generatedId, err := q.beforeInsert(data)
	if err != nil {
		return nil, fmt.Errorf("before insert: %w", err)
	}
	if err := q.callBeforeInsert(data); err != nil {
		return nil, err
	}
	columns := make([]string, 0, len(q.meta.fieldsInfoMap))
	placeholders := make([]string, 0, len(q.meta.fieldsInfoMap))
	args := make([]any, 0, len(q.meta.fieldsInfoMap))
	dataValue := reflect.ValueOf(data).Elem()
	for _, fieldInfo := range q.meta.fieldsInfoMap {
		if fieldInfo.dbIgnore {
			continue
		}
		fieldValue := dataValue.Field(fieldInfo.index)
		if fieldValue.IsZero() {
			continue
		}
		columns = append(columns, fieldInfo.dbColumn)
		placeholders = append(placeholders, "?")
		args = append(args, fieldValue.Interface())
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("%w: table %s", ErrNoFieldsToUpdate, q.getTableName())
	}
	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", q.getTableName(), strings.Join(columns, ", "), strings.Join(placeholders, ", "))
	if q.config.Dialect.DriverName() != "mysql" && q.config.Dialect.DriverName() != "sqlite" {
		sqlStr = q.config.Dialect.ConvertPlaceholders(sqlStr)
	}
	debugLog(q.config, q.ctx, sqlStr, args)
	var result sql.Result
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("insert failed for table %s: %w", q.getTableName(), err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, fmt.Errorf("insert failed: no rows affected for table %s", q.getTableName())
	}
	if q.config.Debug && generatedId != nil {
		fmt.Printf("[DEBUG] Generated ID: %v\n", generatedId)
	}
	if err := q.callAfterInsert(data, result); err != nil {
		return nil, err
	}
	return result, nil
}

// InsertBatch inserts multiple records in a single SQL statement (max 1000).
func (q *DbWrapper[T]) InsertBatch(data []T) (sql.Result, error) {
	if data == nil {
		return nil, ErrNilEntity
	}
	if len(data) == 0 {
		return nil, ErrEmptyData
	}
	const maxBatchSize = 1000
	if len(data) > maxBatchSize {
		return nil, ErrBatchTooLarge
	}
	if q.meta.tableIdDbColumn == "" {
		return nil, ErrNoPrimaryKey
	}
	for i := range data {
		fn, ok := idGenerator[q.meta.idGenerator]
		if !ok {
			return nil, fmt.Errorf("dbw: id generator %s not found", q.meta.idGenerator)
		}
		id := fn()
		idFieldInfo := q.meta.fieldsInfoMap[q.meta.tableIdFieldName]
		reflect.ValueOf(&data[i]).Elem().Field(idFieldInfo.index).Set(reflect.ValueOf(id))
		_, err := q.beforeInsert(&data[i])
		if err != nil {
			return nil, fmt.Errorf("before insert: %w", err)
		}
	}
	dbColumns := make([]string, 0, len(q.meta.dbColumnSlice))
	rowPlaceholders := make([]string, 0, len(data))
	args := make([]any, 0, len(data)*len(q.meta.dbColumnSlice))
	for i := range data {
		rowPhs := make([]string, 0, len(q.meta.dbColumnSlice))
		for _, dbColumn := range q.meta.dbColumnSlice {
			fieldName := q.meta.dbColumnFieldNameMap[dbColumn]
			fi := q.meta.fieldsInfoMap[fieldName]
			if i == 0 {
				if fi.dbIgnore {
					continue
				}
				dbColumns = append(dbColumns, dbColumn)
			}
			if fi.dbIgnore {
				continue
			}
			fieldValue := reflect.ValueOf(&data[i]).Elem().Field(fi.index)
			args = append(args, fieldValue.Interface())
			rowPhs = append(rowPhs, "?")
		}
		rowPlaceholders = append(rowPlaceholders, "("+strings.Join(rowPhs, ", ")+")")
	}
	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", q.getTableName(), strings.Join(dbColumns, ", "), strings.Join(rowPlaceholders, ", "))
	if q.config.Dialect.DriverName() != "mysql" && q.config.Dialect.DriverName() != "sqlite" {
		sqlStr = q.config.Dialect.ConvertPlaceholders(sqlStr)
	}
	debugLog(q.config, q.ctx, sqlStr, args)
	var result sql.Result
	var err error
	if q.tx != nil {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("insert batch failed for table %s: %w", q.getTableName(), err)
	}
	return result, nil
}

// InsertBatchSplit inserts records in batches of the given size.
func (q *DbWrapper[T]) InsertBatchSplit(data []T, size int) ([]sql.Result, error) {
	split, err := sliceSplit(data, size)
	if err != nil {
		return nil, fmt.Errorf("split data: %w", err)
	}
	results := make([]sql.Result, 0, len(split))
	for _, batch := range split {
		result, err := q.InsertBatch(batch)
		if err != nil {
			return results, fmt.Errorf("insert batch: %w", err)
		}
		results = append(results, result)
	}
	return results, nil
}
