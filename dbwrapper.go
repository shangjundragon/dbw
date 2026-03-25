package dbw

import (
	"context"
	"database/sql"
)

// DbWrapper 简化的包装器
type DbWrapper[T any] struct {
	config           *Config
	tx               *sql.Tx
	ctx              context.Context // 上下文
	tableName        string          // 表名
	selects          []string        // 选择的字段
	wheres           []whereExpr     // WHERE条件
	orders           []orderExpr     // 排序
	groupBy          []string        // 分组
	havings          []whereExpr     // HAVING条件
	pageNum          *int            // 页码
	pageSize         *int            // 每页大小
	distinct         bool            // 是否去重
	whereIsOrIndexes map[int]any     // WHERE条件中OR的索引
	meta             *structMeta     // 结构体元数据
}

type Options func(opts *DbWrapper[any])

func WithConfig(config *Config) Options {
	return func(opts *DbWrapper[any]) {
		opts.config = config
	}
}

func WithContext(ctx context.Context) Options {
	return func(opts *DbWrapper[any]) {
		opts.ctx = ctx
	}
}

func WithTx(tx *sql.Tx) Options {
	return func(opts *DbWrapper[any]) {
		opts.tx = tx
	}
}

type whereExpr struct {
	sql  string // SQL 片段
	args []any  // 参数
	isOr bool   // 是否是 OR 条件
}

type orderExpr struct {
	field string
	order string // ASC, DESC
}

// Tabler 表名接口
type Tabler interface {
	TableName() string
}

// Reset 重置 返回新的实例 保留 config meta 表名 可被 opts 修改
func (q *DbWrapper[T]) Reset(opts ...Options) *DbWrapper[T] {
	n := &DbWrapper[T]{
		config:    q.config,
		meta:      q.meta,
		tableName: q.tableName,
	}
	for _, opt := range opts {
		opt((*DbWrapper[any])(n))
	}
	if n.config == nil {
		n.config = GetDefaultConfig()
	}
	if n.config.Db == nil {
		panic("database not properly configured")
	}
	if n.ctx == nil {
		n.ctx = context.Background()
	}
	if n.selects == nil {
		n.selects = []string{"*"}
	}
	if n.meta == nil {
		n.meta = getStructMeta[T]()
	}

	return n
}

// New 创建包装器
func New[T any](opts ...Options) *DbWrapper[T] {
	q := &DbWrapper[T]{}
	for _, opt := range opts {
		opt((*DbWrapper[any])(q))
	}

	if q.config == nil {
		q.config = GetDefaultConfig()
	}
	if q.config.Db == nil {
		panic("database not properly configured")
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

func (q *DbWrapper[T]) TableName(tableName string) *DbWrapper[T] {
	q.tableName = tableName
	return q
}

func (q *DbWrapper[T]) getTableName() string {
	if q.tableName == "" {
		return q.meta.tableName
	}
	return q.tableName
}

func (q *DbWrapper[T]) Clean() *DbWrapper[T] {
	return &DbWrapper[T]{
		ctx:     q.ctx,
		meta:    q.meta,
		selects: []string{"*"},
	}
}

// Tx 设置事务
func (q *DbWrapper[T]) Tx(tx *sql.Tx) *DbWrapper[T] {
	q.tx = tx
	return q
}

// ExecuteTx 执行事务
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

// Select 选择字段
func (q *DbWrapper[T]) Select(fields ...string) *DbWrapper[T] {
	if len(fields) > 0 {
		q.selects = fields
	}
	return q
}

// OrderBy 排序
func (q *DbWrapper[T]) OrderBy(field string) *DbWrapper[T] {
	q.orders = append(q.orders, orderExpr{field: field, order: "ASC"})
	return q
}

// OrderByDesc 降序排序
func (q *DbWrapper[T]) OrderByDesc(field string) *DbWrapper[T] {
	q.orders = append(q.orders, orderExpr{field: field, order: "DESC"})
	return q
}

// GroupBy 分组
func (q *DbWrapper[T]) GroupBy(fields ...string) *DbWrapper[T] {
	q.groupBy = append(q.groupBy, fields...)
	return q
}

// Having HAVING条件
func (q *DbWrapper[T]) Having(sql string, args ...any) *DbWrapper[T] {
	q.havings = append(q.havings, whereExpr{sql: sql, args: args})
	return q
}

// Distinct 去重
func (q *DbWrapper[T]) Distinct() *DbWrapper[T] {
	q.distinct = true
	return q
}

// WithContext 设置上下文
func (q *DbWrapper[T]) WithContext(ctx context.Context) *DbWrapper[T] {
	q.ctx = ctx
	return q
}

func (q *DbWrapper[T]) Clone() *DbWrapper[T] {
	qCopy := *q
	// 深拷贝切片，避免共享底层数组
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
	if qCopy.whereIsOrIndexes != nil {
		qCopy.whereIsOrIndexes = make(map[int]any, len(qCopy.whereIsOrIndexes))
		for k, v := range q.whereIsOrIndexes {
			qCopy.whereIsOrIndexes[k] = v
		}
	}
	return &qCopy
}

// Count 计数
func (q *DbWrapper[T]) Count() (int64, error) {
	// 创建副本执行count查询
	qCopy := q.Clone()
	qCopy.selects = []string{"COUNT(*)"}
	qCopy.orders = nil // count查询不需要order by

	var count int64
	err := qCopy.queryRow().Scan(&count)
	return count, err
}

// Exist 是否存在
func (q *DbWrapper[T]) Exist() (bool, error) {
	count, err := q.Count()
	return count > 0, err
}
