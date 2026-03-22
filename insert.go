package dbw

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// beforeInsert 插入前处理，返回生成的主键值和错误
func (q *DbWrapper[T]) beforeInsert(data *T) (generateTableId any, err error) {
	valueOf := reflect.ValueOf(data).Elem()

	for _, fieldInfo := range q.meta.fieldsInfoMap {
		fieldValue := valueOf.Field(fieldInfo.index)

		// 主键处理
		if fieldInfo.colName == q.meta.tableIdDbColumn {
			generateTableId, err = q.handlePrimaryKey(data, fieldInfo, fieldValue)
			continue
		}

		// 自动时间戳处理
		if err = q.handleAutoTime(data, fieldInfo); err != nil {
			return nil, err
		}

		// 逻辑删除值处理
		if err = q.handleLogicDelete(data, fieldInfo); err != nil {
			return nil, err
		}

		// 零值默认值处理
		if err = q.handleDefaultValue(data, fieldInfo, fieldValue); err != nil {
			return nil, err
		}
	}

	return generateTableId, err
}

// handlePrimaryKey 处理主键逻辑
func (q *DbWrapper[T]) handlePrimaryKey(data *T, fieldInfo fieldInfo, fieldValue reflect.Value) (any, error) {
	if fieldValue.IsZero() {
		if fieldInfo.dbwTag["idType"] == "autoIncrement" {
			return nil, nil // 自增主键不需要生成
		}
		generateTableId := idGenerator[q.meta.idGenerator]()
		err := editStructProp(data, fieldInfo.name, generateTableId)
		return generateTableId, err
	}
	return nil, nil // 已设置主键值
}

// handleAutoTime 处理自动时间戳
func (q *DbWrapper[T]) handleAutoTime(data *T, fieldInfo fieldInfo) error {
	if timeValue := fieldInfo.dbwTag["autoCreateTime"]; timeValue != "" {
		return editStructProp(data, fieldInfo.name, getTime(timeValue))
	}
	if timeValue := fieldInfo.dbwTag["autoUpdateTime"]; timeValue != "" {
		return editStructProp(data, fieldInfo.name, getTime(timeValue))
	}
	return nil
}

// handleLogicDelete 处理逻辑删除值
func (q *DbWrapper[T]) handleLogicDelete(data *T, fieldInfo fieldInfo) error {
	if q.meta.isLogicDelete && fieldInfo.colName == q.meta.logicDelDbColumn {
		return editStructProp(data, fieldInfo.name, q.config.LogicNotDeleteValue)
	}
	return nil
}

// handleDefaultValue 处理零值默认值
func (q *DbWrapper[T]) handleDefaultValue(data *T, fieldInfo fieldInfo, fieldValue reflect.Value) error {
	if fieldValue.IsZero() {
		if defaultValue, has := fieldInfo.dbwTag["default"]; has {
			return editStructProp(data, fieldInfo.name, defaultValue)
		}
	}
	return nil
}

// Insert 插入数据，返回受影响行数
func (q *DbWrapper[T]) Insert(data *T) (sql.Result, error) {
	if data == nil {
		return nil, fmt.Errorf("entity cannot be nil")
	}

	if _, err := q.beforeInsert(data); err != nil {
		return nil, err
	}

	columns, placeholders, args := q.buildInsertValues(data)

	if len(columns) == 0 {
		return nil, fmt.Errorf("no fields to insert")
	}

	sqlStr := q.buildInsertSQL(columns, placeholders)

	if q.config.Debug {
		q.PrintDebugSql(sqlStr, args)
	}

	return q.executeSQL(sqlStr, args)
}

// buildInsertValues 构建插入的列名、占位符和参数
func (q *DbWrapper[T]) buildInsertValues(data *T) ([]string, []string, []any) {
	dataValue := reflect.ValueOf(data).Elem()
	columns := make([]string, 0, len(q.meta.fieldsInfoMap))
	placeholders := make([]string, 0, len(q.meta.fieldsInfoMap))
	args := make([]any, 0, len(q.meta.fieldsInfoMap))

	index := 0
	for _, fieldInfo := range q.meta.fieldsInfoMap {
		fieldValue := dataValue.Field(fieldInfo.index)
		if fieldValue.IsZero() {
			continue
		}

		columns = append(columns, fieldInfo.colName)
		args = append(args, fieldValue.Interface())

		switch q.config.DriverName {
		case "postgres":
			placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
		default:
			placeholders = append(placeholders, "?")
		}
		index++
	}

	return columns, placeholders, args
}

// buildInsertSQL 构建 INSERT SQL 语句
func (q *DbWrapper[T]) buildInsertSQL(columns, placeholders []string) string {
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s);",
		q.getTableName(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)
}

// executeSQL 执行 SQL 语句
func (q *DbWrapper[T]) executeSQL(sqlStr string, args []any) (sql.Result, error) {
	var result sql.Result
	var err error

	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	}

	if err != nil {
		return nil, fmt.Errorf("insert failed: %w", err)
	}

	return result, nil
}

// InsertBatch 批量插入数据，返回受影响行数
func (q *DbWrapper[T]) InsertBatch(data []T) (sql.Result, error) {
	if data == nil {
		return nil, fmt.Errorf("entity cannot be nil")
	}
	if q.meta.tableIdDbColumn == "" {
		return nil, fmt.Errorf("table id column not found")
	}

	// 验证主键类型
	tableIdFieldInfo := q.meta.fieldsInfoMap[q.meta.tableIdProp]
	if idType := tableIdFieldInfo.dbwTag["idType"]; idType != "" && idType != "assign" {
		return nil, fmt.Errorf("primary key type must be 'assign' when inserting multiple records")
	}

	// 执行 beforeInsert 并检查主键唯一性
	generatedIds := make(map[any]struct{}, len(data))
	for i := range data {
		generateTableId, err := q.beforeInsert(&data[i])
		if err != nil {
			return nil, err
		}
		if generateTableId != nil {
			if _, exists := generatedIds[generateTableId]; exists {
				return nil, fmt.Errorf("primary key must be unique when inserting multiple records")
			}
			generatedIds[generateTableId] = struct{}{}
		}
	}

	// 构建批量插入数据
	columns, placeholderRows, args := q.buildBatchInsertValues(data)

	if len(columns) == 0 {
		return nil, fmt.Errorf("no fields to insert")
	}

	sqlStr := q.buildBatchInsertSQL(columns, placeholderRows)

	if q.config.Debug {
		q.PrintDebugSql(sqlStr, args)
	}

	return q.executeSQL(sqlStr, args)
}

// buildBatchInsertValues 构建批量插入的列名、占位符和参数
func (q *DbWrapper[T]) buildBatchInsertValues(data []T) ([]string, []string, []any) {
	columns := make([]string, 0, len(q.meta.dbColumnSlice))
	placeholderRows := make([]string, 0, len(data))
	args := make([]any, 0, len(data)*len(q.meta.dbColumnSlice))

	index := 0
	for i := range data {
		placeholders := make([]string, 0, len(q.meta.dbColumnSlice))
		dataValue := reflect.ValueOf(&data[i]).Elem()

		for _, colName := range q.meta.dbColumnSlice {
			if i == 0 {
				columns = append(columns, colName)
			}

			fieldName := q.meta.dbColumnFieldMap[colName]
			fieldValue := dataValue.FieldByName(fieldName)
			args = append(args, fieldValue.Interface())

			switch q.config.DriverName {
			case "postgres":
				placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
			default:
				placeholders = append(placeholders, "?")
			}
			index++
		}

		placeholderRows = append(placeholderRows, strings.Join(placeholders, ", "))
	}

	return columns, placeholderRows, args
}

// buildBatchInsertSQL 构建批量 INSERT SQL 语句
func (q *DbWrapper[T]) buildBatchInsertSQL(columns, placeholderRows []string) string {
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s);",
		q.getTableName(),
		strings.Join(columns, ", "),
		strings.Join(placeholderRows, "), ("),
	)
}

// InsertBatchSplit 分批批量插入数据
func (q *DbWrapper[T]) InsertBatchSplit(data []T, size int) ([]sql.Result, error) {
	if size <= 0 {
		return nil, fmt.Errorf("batch size must be positive, got %d", size)
	}

	splitData, err := sliceSplit(data, size)
	if err != nil {
		return nil, err
	}

	results := make([]sql.Result, 0, len(splitData))
	for _, batch := range splitData {
		result, err := q.InsertBatch(batch)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}

	return results, nil
}

// InsertByMap 通过 Map 插入数据
func (q *DbWrapper[T]) InsertByMap(dataMap map[string]any) (sql.Result, error) {
	if len(dataMap) == 0 {
		return nil, fmt.Errorf("data map cannot be empty")
	}

	columns, placeholders, args := q.buildMapInsertValues(dataMap)
	sqlStr := q.buildInsertSQL(columns, placeholders)

	if q.config.Debug {
		q.PrintDebugSql(sqlStr, args)
	}

	return q.executeSQL(sqlStr, args)
}

// buildMapInsertValues 构建 Map 插入的列名、占位符和参数
func (q *DbWrapper[T]) buildMapInsertValues(dataMap map[string]any) ([]string, []string, []any) {
	columns := make([]string, 0, len(dataMap))
	placeholders := make([]string, 0, len(dataMap))
	args := make([]any, 0, len(dataMap))

	index := 0
	for colName, value := range dataMap {
		columns = append(columns, colName)
		args = append(args, value)

		switch q.config.DriverName {
		case "postgres":
			placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
		default:
			placeholders = append(placeholders, "?")
		}
		index++
	}

	return columns, placeholders, args
}
