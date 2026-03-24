package dbw

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

func (q *DbWrapper[T]) UpdateById(data *T) (result sql.Result, err error) {

	if q.meta.tableIdFiledName == "" {
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
		// 跳过 ID、逻辑删除字段和忽略字段
		if fieldInfo.dbIgnore || fieldInfo.name == q.meta.tableIdFiledName || fieldInfo.name == q.meta.logicDelFiledName {
			continue
		}
		// 自动更新时间
		if fieldInfo.dbwTag["autoUpdateTime"] != "" {
			if fieldInfo.dbwTag["autoUpdateTime"] == "milli" {
				appendSet(fieldInfo.dbColumn, time.Now().UnixMilli())
			} else {
				appendSet(fieldInfo.dbColumn, time.Now())
			}
			continue
		}

		fieldValue := elem.Field(fieldInfo.index)

		// 检查是否有默认值标签
		defaultValue, hasDefault := fieldInfo.dbwTag["default"]

		// 指针类型：只有明确设置了值（非 nil）才更新
		if fieldValue.Kind() == reflect.Ptr {
			if !fieldValue.IsNil() {
				appendSet(fieldInfo.dbColumn, fieldValue.Interface())
			} else if fieldInfo.dbwTag["tableUpdateStrategy"] == "always" {
				// 策略为总是参与更新，设置为 nil
				appendSet(fieldInfo.dbColumn, nil)
			}
		} else {
			// 值类型：非零值或者零值但有默认值标签时更新
			if !fieldValue.IsZero() {
				appendSet(fieldInfo.dbColumn, fieldValue.Interface())
			} else if fieldInfo.dbwTag["tableUpdateStrategy"] == "always" {
				// 策略为总是参与更新
				if hasDefault {
					// 有默认值则使用默认值
					appendSet(fieldInfo.dbColumn, defaultValue)
				} else {
					// 否则使用零值
					appendSet(fieldInfo.dbColumn, fieldValue.Interface())
				}
			}
		}
	}

	if setCount == 0 {
		return nil, fmt.Errorf("no fields for set")
	}

	sqlStr.WriteString(strings.Join(sets, ", "))
	// 根据拼接id条件
	q.Eq(q.meta.tableIdDbColumn, elem.FieldByName(q.meta.tableIdFiledName).Interface())
	// 逻辑删除条件
	q.AndIf(q.meta.logicDelDbColumn != "", func(w *DbWrapper[T]) {
		w.Eq(q.meta.logicDelDbColumn, q.config.LogicNotDeleteValue)
	})
	whereStr, whereArgs := q.BuildWhere()
	sqlStr.WriteString(whereStr)
	args = append(args, whereArgs...)
	var converterSql string
	if q.config.PlaceholderConverter != nil {
		converterSql = q.config.PlaceholderConverter(sqlStr.String())
	}
	if q.config.Debug {
		q.PrintDebugSql(converterSql, args)
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
		return nil, fmt.Errorf("update without WHERE is dangerous， please add WHERE condition")
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
	q.AndIf(q.meta.logicDelDbColumn != "", func(w *DbWrapper[T]) {
		w.Eq(q.meta.logicDelDbColumn, q.config.LogicNotDeleteValue)
	})
	// WHERE部分的sql和参数
	whereSqlStr, whereArgs := q.BuildWhere()
	sqlStr.WriteString(whereSqlStr)
	args = append(args, whereArgs...)

	var converterSql string
	if q.config.PlaceholderConverter != nil {
		converterSql = q.config.PlaceholderConverter(sqlStr.String())
	}
	if q.config.Debug {
		q.PrintDebugSql(converterSql, args)
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
