package dbw

import (
	"database/sql"
	"fmt"
)

// Delete deletes records matching the WHERE conditions.
func (q *DbWrapper[T]) Delete() (sql.Result, error) {
	if len(q.wheres) == 0 {
		return nil, ErrNoWhereClause
	}
	if err := q.callEntityHook(HookBeforeDelete, nil); err != nil {
		return nil, err
	}
	if err := q.callBeforeDelete(); err != nil {
		return nil, err
	}
	sqlStr, args := q.buildDeleteSQL()
	debugLog(q.config, q.ctx, sqlStr, args)
	var result sql.Result
	var err error
	if q.tx == nil {
		result, err = q.config.Db.ExecContext(q.ctx, sqlStr, args...)
	} else {
		result, err = q.tx.ExecContext(q.ctx, sqlStr, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("delete failed: %w", err)
	}
	if err := q.callAfterDelete(result); err != nil {
		return nil, err
	}
	if err := q.callEntityHook(HookAfterDelete, nil); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteById deletes a record by its primary key.
func (q *DbWrapper[T]) DeleteById(id any) (sql.Result, error) {
	if q.meta.tableIdFieldName == "" {
		return nil, ErrNoPrimaryKey
	}
	q.Eq(q.meta.tableIdDbColumn, id)
	return q.Delete()
}

// DeleteByIds deletes records by their primary keys.
func (q *DbWrapper[T]) DeleteByIds(ids []any) (sql.Result, error) {
	if len(ids) == 0 {
		return nil, ErrEmptyData
	}
	if q.meta.tableIdFieldName == "" {
		return nil, ErrNoPrimaryKey
	}
	q.In(q.meta.tableIdDbColumn, ids)
	return q.Delete()
}
