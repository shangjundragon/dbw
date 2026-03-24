package dbw

import (
	"database/sql"
	"fmt"
	"strings"
)

// Delete 删除 返回受影响的行数和错误
func (q *DbWrapper[T]) Delete() (result sql.Result, err error) {
	if len(q.wheres) == 0 {
		return nil, fmt.Errorf("delete without WHERE is not allowed")
	}

	// 逻辑删除
	if q.meta.logicDelDbColumn != "" {
		sets := map[string]any{q.meta.logicDelDbColumn: q.config.LogicDeleteValue}
		result, err = q.Update(sets)
		return result, err
	}
	sqlStr := strings.Builder{}

	sqlStr.WriteString("DELETE FROM " + q.getTableName())
	whereStr, args := q.BuildWhere()
	sqlStr.WriteString(whereStr)

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

// DeleteById 根据id删除
func (q *DbWrapper[T]) DeleteById(id any) (result sql.Result, err error) {
	if q.meta.tableIdFiledName == "" {
		return nil, fmt.Errorf("table id property not found")
	}
	q.Eq(q.meta.tableIdDbColumn, id)
	return q.Delete()
}

// DeleteByIds 批量根据id删除
func (q *DbWrapper[T]) DeleteByIds(ids []any) (result sql.Result, err error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if q.meta.tableIdFiledName == "" {
		return nil, fmt.Errorf("table id property not found")
	}
	q.In(q.meta.tableIdDbColumn, ids)
	return q.Delete()
}
