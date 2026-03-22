package dbw

import (
	"database/sql"
	"fmt"
	"strings"
)

// Delete 删除 返回受影响的行数和错误
func (q *DbWrapper[T]) Delete() (rowsAffected int64, err error) {
	if len(q.wheres) == 0 {
		return 0, fmt.Errorf("delete without WHERE is not allowed")
	}

	// 逻辑删除
	if q.meta.isLogicDelete {
		sets := map[string]any{q.meta.logicDelDbColumn: q.config.LogicDeleteValue}
		rowsAffected, err = q.Update(sets)
		return rowsAffected, err
	}
	sqlStr := strings.Builder{}

	sqlStr.WriteString("DELETE FROM " + q.getTableName())
	whereStr, args := q.BuildWhere()
	sqlStr.WriteString(whereStr)

	if q.config.Debug {
		q.PrintDebugSql(sqlStr.String(), args)
	}
	var result sql.Result
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr.String(), args...)

	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr.String(), args...)
	}
	if err != nil {
		return 0, err
	}
	affected, e := result.RowsAffected()
	if e != nil {
		return 0, e
	}
	return affected, nil
}

// DeleteById 根据id删除
func (q *DbWrapper[T]) DeleteById(id any) (rowsAffected int64, err error) {
	if q.meta.tableIdProp == "" {
		return 0, fmt.Errorf("table id property not found")
	}
	q.Eq(q.meta.tableIdDbColumn, id)
	return q.Delete()
}

// DeleteByIds 批量根据id删除
func (q *DbWrapper[T]) DeleteByIds(ids []any) (rowsAffected int64, err error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if q.meta.tableIdProp == "" {
		return 0, fmt.Errorf("table id property not found")
	}
	q.In(q.meta.tableIdDbColumn, ids)
	return q.Delete()
}
