package dbw

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// beforeInsert 插入前处理 返回 是否生成了主键、生成的主键值
func (q *DbWrapper[T]) beforeInsert(data *T) (generateTableId any, err error) {
	valueOf := reflect.ValueOf(data).Elem()

	// 遍历结构体字段
	for _, fieldInfo := range q.meta.fieldsInfoMap {
		fieldValue := valueOf.Field(fieldInfo.index)

		// 主键处理
		if fieldInfo.colName == q.meta.tableIdDbColumn {
			if fieldValue.IsZero() {
				generateTableId = idGenerator[q.meta.idGenerator]()
				err = editStructProp(data, fieldInfo.name, generateTableId)
			}
			continue
		}

		// 创建时间处理
		if autoCreateTime := fieldInfo.dbwTag["autoCreateTime"]; autoCreateTime != "" {
			err = editStructProp(data, fieldInfo.name, getTime(autoCreateTime))
			continue
		}

		// 更新时间处理
		if autoUpdateTime := fieldInfo.dbwTag["autoUpdateTime"]; autoUpdateTime != "" {
			err = editStructProp(data, fieldInfo.name, getTime(autoUpdateTime))
			continue
		}

		// 逻辑删除值处理
		if q.meta.logicDelDbColumn != "" && fieldInfo.colName == q.meta.logicDelDbColumn {
			err = editStructProp(data, fieldInfo.name, q.config.LogicNotDeleteValue)
			continue
		}

		// 零值处理 - 设置默认值
		if fieldValue.IsZero() {
			if defaultValue, has := fieldInfo.dbwTag["default"]; has {
				err = editStructProp(data, fieldInfo.name, defaultValue)
			}
		}
	}
	return generateTableId, err
}

// Insert 插入数据 返回受影响行数
func (q *DbWrapper[T]) Insert(data *T) (result sql.Result, err error) {
	if data == nil {
		return nil, fmt.Errorf("entity cannot be nil")
	}

	_, err = q.beforeInsert(data)
	if err != nil {
		return nil, fmt.Errorf("before insert failed: %w", err)
	}

	// 预分配切片容量
	columns := make([]string, 0, len(q.meta.fieldsInfoMap))
	placeholders := make([]string, 0, len(q.meta.fieldsInfoMap))
	args := make([]any, 0, len(q.meta.fieldsInfoMap))

	// 反射数据值
	dataValue := reflect.ValueOf(data).Elem()

	// 构建列名、占位符和参数
	for _, fieldInfo := range q.meta.fieldsInfoMap {
		fieldValue := dataValue.Field(fieldInfo.index)
		// 跳过零值
		if fieldValue.IsZero() {
			continue
		}
		columns = append(columns, fieldInfo.colName)
		placeholders = append(placeholders, "?")
		args = append(args, fieldValue.Interface())
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("no fields to insert")
	}

	// 构建 INSERT 语句
	sqlStr := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		q.getTableName(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	// 转换占位符
	if q.config.PlaceholderConverter != nil {
		sqlStr = q.config.PlaceholderConverter(sqlStr)
	}

	// 打印调试信息
	if q.config.Debug {
		q.PrintDebugSql(sqlStr, args)
	}

	// 执行插入
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	}

	if err != nil {
		return nil, fmt.Errorf("insert failed: %w", err)
	}

	// 检查受影响行数
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("insert failed: %w", err)
	}
	if affected == 0 {
		return nil, fmt.Errorf("insert failed: no rows affected")
	}

	return result, nil
}

// InsertBatch 批量插入数据
func (q *DbWrapper[T]) InsertBatch(data []T) (result sql.Result, err error) {
	if data == nil {
		return nil, fmt.Errorf("entity cannot be nil")
	}
	if q.meta.tableIdDbColumn == "" {
		return nil, fmt.Errorf("table id column not found")
	}

	// 验证主键类型
	tableIdFieldInfo := q.meta.fieldsInfoMap[q.meta.tableIdProp]
	idType := tableIdFieldInfo.dbwTag["idType"]
	if idType != "" && idType != "assign" {
		return nil, fmt.Errorf("primary key type must be 'assign' when inserting multiple records")
	}

	// 生成主键并检查重复
	generateTableIdMap := make(map[any]struct{}, len(data))
	for i := range data {
		generateTableId, err := q.beforeInsert(&data[i])
		if err != nil {
			return nil, fmt.Errorf("before insert failed: %w", err)
		}
		if _, exists := generateTableIdMap[generateTableId]; exists {
			return nil, fmt.Errorf("primary key must be unique when inserting multiple records")
		}
		generateTableIdMap[generateTableId] = struct{}{}
	}

	// 预分配切片容量
	columns := make([]string, 0, len(q.meta.dbColumnSlice))
	placeholders := make([]string, 0, len(data))
	args := make([]any, 0, len(data)*len(q.meta.dbColumnSlice))

	// 构建列名和参数
	for i := range data {
		rowPlaceholders := make([]string, 0, len(q.meta.dbColumnSlice))
		for _, colName := range q.meta.dbColumnSlice {
			if i == 0 {
				columns = append(columns, colName)
			}
			fieldName := q.meta.dbColumnFieldMap[colName]
			fieldValue := reflect.ValueOf(&data[i]).Elem().FieldByName(fieldName)
			args = append(args, fieldValue.Interface())
			rowPlaceholders = append(rowPlaceholders, "?")
		}
		placeholders = append(placeholders, strings.Join(rowPlaceholders, ", "))
	}

	// 构建 INSERT 语句
	sqlStr := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		q.getTableName(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, "), ("),
	)

	// 转换占位符
	if q.config.PlaceholderConverter != nil {
		sqlStr = q.config.PlaceholderConverter(sqlStr)
	}

	// 打印调试信息
	if q.config.Debug {
		q.PrintDebugSql(sqlStr, args)
	}

	// 执行插入
	if q.tx != nil {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	}

	if err != nil {
		return nil, fmt.Errorf("insert batch failed: %w", err)
	}

	return result, nil
}

// InsertBatchSplit 分批批量插入数据
func (q *DbWrapper[T]) InsertBatchSplit(data []T, size int) (results []sql.Result, err error) {
	// 分割数据
	split, err := sliceSplit(data, size)
	if err != nil {
		return nil, fmt.Errorf("split data failed: %w", err)
	}

	// 预分配结果切片
	results = make([]sql.Result, 0, len(split))

	// 分批插入
	for _, batch := range split {
		result, err := q.InsertBatch(batch)
		if err != nil {
			return results, fmt.Errorf("insert batch failed: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}
