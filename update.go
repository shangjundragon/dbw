package dbw

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

func (q *DbWrapper[T]) UpdateById(data *T) (result sql.Result, err error) {

	if q.meta.tableIdProp == "" {
		return nil, fmt.Errorf("table id property not found")
	}
	sqlStr := strings.Builder{}
	args := make([]any, 0)
	sqlStr.WriteString("UPDATE " + q.getTableName() + " SET ")
	sets := make([]string, 0, len(q.meta.fieldsInfoMap))
	elem := reflect.ValueOf(data).Elem()
	setCount := 0
	appendSet := func(colName string, arg any) {
		sets = append(sets, colName+" = ?")
		args = append(args, arg)
		setCount++
	}
	for _, fieldInfo := range q.meta.fieldsInfoMap {
		// 跳过ID和逻辑删除字段
		if fieldInfo.name == q.meta.tableIdProp || fieldInfo.name == q.meta.logicDelProp {
			continue
		}
		// 自动更新时间
		if fieldInfo.dbwTag["autoUpdateTime"] != "" {
			if fieldInfo.dbwTag["autoUpdateTime"] == "milli" {
				appendSet(fieldInfo.colName, time.Now().UnixMilli())

			} else {
				appendSet(fieldInfo.colName, time.Now())
			}
			continue
		}

		fieldValue := elem.Field(fieldInfo.index)
		// 非零值
		if !fieldValue.IsZero() {
			appendSet(fieldInfo.colName, fieldValue.Interface())
			continue
		}

		// 指针类型
		if fieldValue.Kind() == reflect.Ptr {
			// 更新策略为总是参与更新
			if fieldInfo.dbwTag["tableUpdateStrategy"] == "always" {
				appendSet(fieldInfo.colName, nil)
			}
		}

		// 非指针类型
		if fieldValue.Kind() != reflect.Ptr {
			// 更新策略为总是参与更新
			if fieldInfo.dbwTag["tableUpdateStrategy"] == "always" {
				// 默认值
				defaultZeroValue, has := fieldInfo.dbwTag["default"]
				if has {
					appendSet(fieldInfo.colName, defaultZeroValue)
				} else {
					appendSet(fieldInfo.colName, fieldValue.Interface())
				}
			}
		}

	}
	if setCount == 0 {
		return nil, nil
	}
	sqlStr.WriteString(strings.Join(sets, ", "))
	// id 条件
	q.Eq(q.meta.tableIdDbColumn, elem.FieldByName(q.meta.tableIdProp).Interface())
	// 逻辑删除条件
	q.AndIf(q.meta.isLogicDelete, func(w *DbWrapper[T]) {
		w.Eq(q.meta.logicDelDbColumn, q.config.LogicNotDeleteValue)
	})
	whereStr, whereArgs := q.BuildWhere()
	sqlStr.WriteString(whereStr)
	args = append(args, whereArgs...)

	if q.config.Debug {
		q.PrintDebugSql(sqlStr.String(), args)
	}
	var converterSql string
	if q.config.PlaceholderConverter != nil {
		converterSql = q.config.PlaceholderConverter(sqlStr.String())
	}
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, converterSql, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, converterSql, args...)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Update 更新
func (q *DbWrapper[T]) Update(values map[string]any) (result sql.Result, err error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("no values to update")
	}
	// 警告：没有WHERE条件的更新
	if len(q.wheres) == 0 {
		return nil, fmt.Errorf("update without WHERE is dangerous, use UpdateAll if you really want to update all rows")
	}
	// 自动更新时间
	if q.meta.autoUpdateTimeDbColumn != "" {
		if q.meta.autoUpdateTimeTagValue == "milli" {
			values[q.meta.autoUpdateTimeDbColumn] = time.Now().UnixMilli()
		} else {
			values[q.meta.autoUpdateTimeDbColumn] = time.Now()
		}
	}

	sqlStr := strings.Builder{}
	args := make([]any, 0)

	sqlStr.WriteString("UPDATE " + q.getTableName() + " SET ")

	sets := make([]string, 0, len(values))
	for k, v := range values {
		sets = append(sets, k+" = ?")
		args = append(args, v)
	}
	sqlStr.WriteString(strings.Join(sets, ", "))
	// 逻辑删除
	q.AndIf(q.meta.isLogicDelete, func(w *DbWrapper[T]) {
		w.Eq(q.meta.logicDelDbColumn, q.config.LogicNotDeleteValue)
	})
	// WHERE
	str, anies := q.BuildWhere()
	sqlStr.WriteString(str)
	args = append(args, anies...)

	if q.config.Debug {
		q.PrintDebugSql(sqlStr.String(), args)
	}

	var converterSql string
	if q.config.PlaceholderConverter != nil {
		converterSql = q.config.PlaceholderConverter(sqlStr.String())
	}
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, converterSql, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, converterSql, args...)
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}
