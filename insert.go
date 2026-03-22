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
				if fieldInfo.dbwTag["idType"] == "autoIncrement" {
					// 自增主键
					continue
				} else {
					generateTableId = idGenerator[q.meta.idGenerator]()
					err = editStructProp(data, fieldInfo.name, generateTableId)

					continue
				}
			} else {
				// 调用方已设置主键值
				continue
			}
		}

		// 创建时间处理
		autoCreateTime := fieldInfo.dbwTag["autoCreateTime"]
		if autoCreateTime != "" {
			err = editStructProp(data, fieldInfo.name, getTime(autoCreateTime))
			continue
		}

		// 更新时间处理
		autoUpdateTime := fieldInfo.dbwTag["autoUpdateTime"]
		if autoUpdateTime != "" {
			err = editStructProp(data, fieldInfo.name, getTime(autoUpdateTime))
			continue
		}

		// 逻辑删除值处理
		if q.meta.isLogicDelete && fieldInfo.colName == q.meta.logicDelDbColumn {
			err = editStructProp(data, fieldInfo.name, q.config.LogicNotDeleteValue)
			continue
		}

		// 零值处
		if fieldValue.IsZero() {
			zeroDefaultValue, has := fieldInfo.dbwTag["default"]
			if has {
				// 如果有default标签 设置default标签的默认值
				err = editStructProp(data, fieldInfo.name, zeroDefaultValue)
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

	// 准备列名和值
	columns := make([]string, 0, len(q.meta.fieldsInfoMap))
	placeholders := make([]string, 0, len(q.meta.fieldsInfoMap))
	args := make([]any, 0, len(q.meta.fieldsInfoMap))

	// 反射数据值
	dataValue := reflect.ValueOf(data).Elem()
	// 定义内部函数，用于添加列名、占位符和参数
	appendValue := func(colName string, val any) {
		columns = append(columns, colName)
		placeholders = append(placeholders, "?")
		args = append(args, val)
	}

	for _, fieldInfo := range q.meta.fieldsInfoMap {
		fieldValue := dataValue.Field(fieldInfo.index)
		// 零值
		if fieldValue.IsZero() {
			continue
		}
		appendValue(fieldInfo.colName, fieldValue.Interface())
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("no fields to insert")
	}

	// 构建INSERT语句
	sqlStr := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		q.getTableName(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	if q.config.Debug {
		q.PrintDebugSql(sqlStr, args)
	}

	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	}

	if err != nil {
		return nil, fmt.Errorf("insert failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("insert failed: %w", err)
	}
	if affected == 0 {
		return nil, fmt.Errorf("insert failed: no rows affected")
	}

	return result, nil
}

// InsertBatch 批量插入数据 返回受影响行数
func (q *DbWrapper[T]) InsertBatch(data []T) (result sql.Result, err error) {
	if data == nil {
		return nil, fmt.Errorf("entity cannot be nil")
	}
	if q.meta.tableIdDbColumn == "" {
		return nil, fmt.Errorf("table id column not found")
	}

	// 必须是生成id
	tableIdFieldInfo := q.meta.fieldsInfoMap[q.meta.tableIdProp]
	idType := tableIdFieldInfo.dbwTag["idType"]
	if idType != "" && idType != "assign" {
		return nil, fmt.Errorf("primary key type must be 'assign' when inserting multiple records")
	}
	var generateTableIdMap = make(map[any]string)
	for i := range data {
		var generateTableId any
		generateTableId, err = q.beforeInsert(&data[i])
		generateTableIdMap[generateTableId] = "1"
	}
	// 检查主键是否重复
	if len(generateTableIdMap) != len(data) {
		return nil, fmt.Errorf("primary key must be unique when inserting multiple records")
	}

	// 准备列名和值
	columns := make([]string, 0)
	placeholders := make([]string, 0)
	args := make([]any, 0)

	for i := range data {
		p := make([]string, 0)
		for _, colName := range q.meta.dbColumnSlice {
			if i == 0 {
				columns = append(columns, colName)
			}
			fieldName := q.meta.dbColumnFieldMap[colName]
			fieldValue := reflect.ValueOf(&data[i]).Elem().FieldByName(fieldName)
			args = append(args, fieldValue.Interface())
			p = append(p, "?")
		}

		placeholders = append(placeholders, strings.Join(p, ", "))
	}

	// 构建INSERT语句
	sqlStr := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		q.getTableName(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, "), ("),
	)
	if q.config.Debug {

		q.PrintDebugSql(sqlStr, args)
	}

	if q.tx != nil {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	}

	if err != nil {
		return nil, err
	}

	return result, err

}

func (q *DbWrapper[T]) InsertBatchSplit(data []T, size int) (results []sql.Result, err error) {

	split, err := sliceSplit(data, size)
	if err != nil {
		return nil, err
	}
	for i := range split {
		result, err := q.InsertBatch(split[i])
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, err

}
