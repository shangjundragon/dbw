package dbw

import (
	"database/sql"
	"reflect"
	"strings"
	"time"
)

// UpdateById updates a record identified by its primary key.
func (q *DbWrapper[T]) UpdateById(data *T) (sql.Result, error) {
	if q.meta.tableIdFieldName == "" {
		return nil, ErrNoPrimaryKey
	}
	var b strings.Builder
	args := make([]any, 0)
	b.WriteString("UPDATE " + q.getTableName() + " SET ")
	sets := make([]string, 0, len(q.meta.fieldsInfoMap))
	if err := q.callBeforeUpdate(data); err != nil {
		return nil, err
	}
	elem := reflect.ValueOf(data).Elem()
	setCount := 0
	appendSet := func(colName string, arg any) {
		sets = append(sets, colName+" = ?")
		args = append(args, arg)
		setCount++
	}
	for _, fieldInfo := range q.meta.fieldsInfoMap {
		if fieldInfo.dbIgnore || fieldInfo.name == q.meta.tableIdFieldName || fieldInfo.name == q.meta.logicDelFieldName {
			continue
		}
		if fieldInfo.dbwTag["autoUpdateTime"] != "" {
			if fieldInfo.dbwTag["autoUpdateTime"] == "milli" {
				appendSet(fieldInfo.dbColumn, time.Now().UnixMilli())
			} else {
				appendSet(fieldInfo.dbColumn, time.Now())
			}
			continue
		}
		fieldValue := elem.Field(fieldInfo.index)
		defaultValue, hasDefault := fieldInfo.dbwTag["default"]
		if fieldValue.Kind() == reflect.Ptr {
			if !fieldValue.IsNil() {
				appendSet(fieldInfo.dbColumn, fieldValue.Interface())
			} else if fieldInfo.dbwTag["tableUpdateStrategy"] == "always" {
				appendSet(fieldInfo.dbColumn, nil)
			}
		} else {
			if !fieldValue.IsZero() {
				appendSet(fieldInfo.dbColumn, fieldValue.Interface())
			} else if fieldInfo.dbwTag["tableUpdateStrategy"] == "always" {
				if hasDefault {
					appendSet(fieldInfo.dbColumn, defaultValue)
				} else {
					appendSet(fieldInfo.dbColumn, fieldValue.Interface())
				}
			}
		}
	}
	if setCount == 0 {
		return nil, ErrNoFieldsToUpdate
	}
	b.WriteString(strings.Join(sets, ", "))
	idVal := elem.FieldByName(q.meta.tableIdFieldName).Interface()
	clone := q.Clone()
	clone.Eq(clone.meta.tableIdDbColumn, idVal)
	clone = clone.cloneForLogicDel()
	whereStr, whereArgs := buildWhere(clone.wheres)
	b.WriteString(whereStr)
	args = append(args, whereArgs...)
	sqlStr := b.String()
	if q.config.Dialect.DriverName() != "mysql" && q.config.Dialect.DriverName() != "sqlite" {
		sqlStr = q.config.Dialect.ConvertPlaceholders(sqlStr)
	}
	debugLog(q.config, q.ctx, sqlStr, args)
	var result sql.Result
	var err error
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	}
	if err != nil {
		return nil, err
	}
	if err := q.callAfterUpdate(result); err != nil {
		return nil, err
	}
	return result, nil
}

// Update updates records matching the WHERE conditions with the given values.
func (q *DbWrapper[T]) Update(values map[string]any) (sql.Result, error) {
	if len(values) == 0 {
		return nil, ErrNoFieldsToUpdate
	}
	if len(q.wheres) == 0 {
		return nil, ErrNoWhereClause
	}
	if err := q.callBeforeUpdateMap(values); err != nil {
		return nil, err
	}
	if q.meta.autoUpdateTimeDbColumn != "" {
		if q.meta.autoUpdateTimeTagValue == "milli" {
			values[q.meta.autoUpdateTimeDbColumn] = time.Now().UnixMilli()
		} else {
			values[q.meta.autoUpdateTimeDbColumn] = time.Now()
		}
	}
	sqlStr, args := q.buildUpdateSQL(values)
	debugLog(q.config, q.ctx, sqlStr, args)
	var result sql.Result
	var err error
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	}
	if err != nil {
		return nil, err
	}
	if err := q.callAfterUpdate(result); err != nil {
		return nil, err
	}
	return result, nil
}
