package handler

import (
	"net/url"
	"strings"
)

// Dialect identifies the SQL flavor a handler speaks. The server supports
// both PostgreSQL and MariaDB; all dialect-specific SQL lives in the query
// builders guarded by h.dialect.
type Dialect int

const (
	DialectPostgres Dialect = iota
	DialectMariadb
)

func (d Dialect) String() string {
	switch d {
	case DialectMariadb:
		return "mariadb"
	default:
		return "postgres"
	}
}

// DialectFromDSN detects the target dialect from a DSN URL scheme.
//   - postgres://, postgresql://  → DialectPostgres
//   - mysql://, mariadb://        → DialectMariadb
//
// Unknown or unparseable schemes default to DialectPostgres
// (backward-compatible with existing postgres DSNs that a URL parser
// might not fully understand, and with non-URL DSNs that historically
// targeted PostgreSQL).
func DialectFromDSN(dsn string) Dialect {
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		return DialectPostgres
	}
	switch strings.ToLower(u.Scheme) {
	case "mysql", "mariadb":
		return DialectMariadb
	default:
		return DialectPostgres
	}
}
