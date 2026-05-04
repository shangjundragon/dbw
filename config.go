package dbw

import (
	"database/sql"
	"time"
)

// Dialect is the database dialect interface for SQL generation.
type Dialect interface {
	DriverName() string
	Placeholder(n int) string
	ConvertPlaceholders(sql string) string
	QuoteIdentifier(name string) string
	BuildPagination(sql string, limit, offset int) string
}

// Config holds the database configuration and dialect settings.
type Config struct {
	Db                  *sql.DB
	DriverName          string // mysql, sqlite, postgres, oracle
	Dialect             Dialect
	LogicDeleteValue    string
	LogicNotDeleteValue string
	Debug               bool
	PageInterceptor     func(sqlStr string, pageNum int, pageSize int) string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration

	PreparedStmtCacheSize int
}

// NewConfig creates a new Config using the provided callback function.
func NewConfig(fn func(config *Config)) *Config {
	c := &Config{
		LogicDeleteValue:    "1",
		LogicNotDeleteValue: "0",
	}
	fn(c)
	if c.Db == nil {
		panic("dbw: database connection is required")
	}
	if c.LogicDeleteValue == "" {
		panic("dbw: logic delete value is required")
	}
	if c.LogicNotDeleteValue == "" {
		panic("dbw: logic not delete value is required")
	}

	if c.Dialect == nil {
		switch c.DriverName {
		case "mysql":
			c.Dialect = &mysqlDialect{}
		case "sqlite":
			c.Dialect = &sqliteDialect{}
		case "postgres":
			c.Dialect = &postgresDialect{}
		case "oracle":
			c.Dialect = &oracleDialect{}
		default:
			c.Dialect = &mysqlDialect{}
		}
	}

	if c.MaxOpenConns > 0 {
		c.Db.SetMaxOpenConns(c.MaxOpenConns)
	}
	if c.MaxIdleConns > 0 {
		c.Db.SetMaxIdleConns(c.MaxIdleConns)
	}
	if c.ConnMaxLifetime > 0 {
		c.Db.SetConnMaxLifetime(c.ConnMaxLifetime)
	}

	return c
}
