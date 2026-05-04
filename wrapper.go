package dbw

import (
	"context"
	"database/sql"
	"fmt"
)

// DbWrapper is a type-safe ORM wrapper for database operations on type T.
type DbWrapper[T any] struct {
	config    *Config
	tx        *sql.Tx
	ctx       context.Context
	tableName string
	selects   []string
	wheres    []whereExpr
	orders    []orderExpr
	groupBy   []string
	havings   []whereExpr
	pageNum   *int
	pageSize  *int
	distinct  bool
	limit     *int
	offset    *int
	rawSQL    string
	rawArgs   []any
	meta      *structMeta
}

// Options is a functional option for configuring a DbWrapper.
type Options func(opts *DbWrapper[any])

// whereExpr represents a single WHERE clause expression.
type whereExpr struct {
	sql    string
	args   []any
	joiner string // "AND" or "OR", empty for first
}

// orderExpr represents an ORDER BY clause expression.
type orderExpr struct {
	field string
	order string
}

// WithConfig sets the database configuration.
func WithConfig(config *Config) Options {
	return func(opts *DbWrapper[any]) { opts.config = config }
}

// WithContext sets the context for database operations.
func WithContext(ctx context.Context) Options {
	return func(opts *DbWrapper[any]) { opts.ctx = ctx }
}

// WithTx sets the transaction for database operations.
func WithTx(tx *sql.Tx) Options {
	return func(opts *DbWrapper[any]) { opts.tx = tx }
}

// New creates a new DbWrapper instance for type T.
func New[T any](opts ...Options) *DbWrapper[T] {
	q := &DbWrapper[T]{}
	for _, opt := range opts {
		opt((*DbWrapper[any])(q))
	}
	if q.config == nil {
		panic("dbw: config is required, use dbw.WithConfig(config)")
	}
	if q.config.Db == nil {
		panic("dbw: database connection is nil")
	}
	if q.ctx == nil {
		q.ctx = context.Background()
	}
	if q.selects == nil {
		q.selects = []string{"*"}
	}
	if q.meta == nil {
		q.meta = getStructMeta[T]()
	}
	return q
}

// Reset creates a new DbWrapper preserving config and meta.
func (q *DbWrapper[T]) Reset(opts ...Options) *DbWrapper[T] {
	n := &DbWrapper[T]{
		config:    q.config,
		meta:      q.meta,
		tableName: q.tableName,
	}
	for _, opt := range opts {
		opt((*DbWrapper[any])(n))
	}
	if n.config == nil || n.config.Db == nil {
		panic("dbw: config is required")
	}
	if n.ctx == nil {
		n.ctx = context.Background()
	}
	if n.selects == nil {
		n.selects = []string{"*"}
	}
	if n.meta == nil {
		n.meta = q.meta
	}
	return n
}

// Clone creates a deep copy of the DbWrapper.
func (q *DbWrapper[T]) Clone() *DbWrapper[T] {
	qCopy := *q
	if qCopy.selects != nil {
		qCopy.selects = append([]string(nil), qCopy.selects...)
	}
	if qCopy.wheres != nil {
		qCopy.wheres = append([]whereExpr(nil), qCopy.wheres...)
	}
	if qCopy.orders != nil {
		qCopy.orders = append([]orderExpr(nil), qCopy.orders...)
	}
	if qCopy.groupBy != nil {
		qCopy.groupBy = append([]string(nil), qCopy.groupBy...)
	}
	if qCopy.havings != nil {
		qCopy.havings = append([]whereExpr(nil), qCopy.havings...)
	}
	qCopy.rawArgs = append([]any(nil), qCopy.rawArgs...)
	return &qCopy
}

// Clean creates a minimal DbWrapper preserving config, ctx, meta, and table name.
func (q *DbWrapper[T]) Clean() *DbWrapper[T] {
	return &DbWrapper[T]{
		config:    q.config,
		ctx:       q.ctx,
		meta:      q.meta,
		tableName: q.tableName,
		selects:   []string{"*"},
	}
}

// TableName overrides the table name for this query.
func (q *DbWrapper[T]) TableName(tableName string) *DbWrapper[T] {
	q.tableName = tableName
	return q
}

func (q *DbWrapper[T]) getTableName() string {
	if q.tableName != "" {
		return q.tableName
	}
	return q.meta.tableName
}

// Tx sets the transaction for this DbWrapper instance.
func (q *DbWrapper[T]) Tx(tx *sql.Tx) *DbWrapper[T] {
	q.tx = tx
	return q
}

// WithContext sets the context for this DbWrapper instance.
func (q *DbWrapper[T]) WithContext(ctx context.Context) *DbWrapper[T] {
	q.ctx = ctx
	return q
}

func (q *DbWrapper[T]) cloneForLogicDel() *DbWrapper[T] {
	clone := q.Clone()
	if clone.meta.logicDelDbColumn != "" && clone.config != nil {
		clone.wheres = append(clone.wheres, whereExpr{
			sql:    fmt.Sprintf("%s = ?", clone.meta.logicDelDbColumn),
			args:   []any{clone.config.LogicNotDeleteValue},
			joiner: "AND",
		})
	}
	return clone
}

// ExecuteTx executes a function within a database transaction.
func ExecuteTx(txFn func(*sql.Tx) error, db *sql.DB) (err error) {
	var tx *sql.Tx
	tx, err = db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()
	err = txFn(tx)
	return err
}
