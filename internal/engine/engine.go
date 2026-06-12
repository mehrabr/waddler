package engine

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/marcboeker/go-duckdb/v2"

	"github.com/mehrabr/waddler/internal/sqlutil"
)

type Engine struct{ db *sql.DB }

// New opens an in-memory DuckDB instance.
func New() (*Engine, error) { return open("") }

// NewWithFile opens a file-backed DuckDB instead of in-memory. The database
// persists after the run, so you can inspect it with the DuckDB CLI if
// something goes wrong.
func NewWithFile(path string) (*Engine, error) { return open(path) }

func open(dsn string) (*Engine, error) {
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return nil, fmt.Errorf("engine: open duckdb: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("engine: connect to duckdb: %w", err)
	}
	return &Engine{db: db}, nil
}

// Exec runs a statement. It deliberately does NOT include the SQL text in the
// error: connection strings and ATTACH statements can contain tokens, and the
// previous version leaked them into error output and logs. Callers add their
// own (non-sensitive) context.
func (e *Engine) Exec(query string, args ...any) error {
	if _, err := e.db.Exec(query, args...); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	return nil
}

// CreateView registers selectSQL as a view. The view name is quoted, so a
// source named `x"; DROP ...` becomes a (harmless) quoted identifier instead of
// executable SQL.
func (e *Engine) CreateView(name, selectSQL string) error {
	return e.Exec(fmt.Sprintf("CREATE OR REPLACE VIEW %s AS %s", sqlutil.QuoteIdent(name), selectSQL))
}

// Materialize runs selectSQL once and stores the result in a temporary table,
// so the transform executes a single time regardless of how many validation
// rules run or what the output is.
func (e *Engine) Materialize(name, selectSQL string) error {
	return e.Exec(fmt.Sprintf(
		"CREATE OR REPLACE TEMP TABLE %s AS SELECT * FROM (%s) AS _waddler_src",
		sqlutil.QuoteIdent(name), selectSQL,
	))
}

// CountTable returns the row count of a materialized table.
func (e *Engine) CountTable(name string) (int64, error) {
	var n int64
	err := e.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", sqlutil.QuoteIdent(name))).Scan(&n)
	return n, err
}

// RowCount returns how many rows a SELECT would produce. Retained for
// compatibility; the runner now materializes the result once and uses
// CountTable instead.
func (e *Engine) RowCount(selectSQL string) (int64, error) {
	var n int64
	return n, e.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS _waddler_src", selectSQL)).Scan(&n)
}

// ScalarInt64 runs a query expected to return a single integer (used by
// validation assertions).
func (e *Engine) ScalarInt64(query string) (int64, error) {
	var n int64
	return n, e.db.QueryRow(query).Scan(&n)
}

// ScalarString runs a query expected to return a single string value.
func (e *Engine) ScalarString(query string) (string, error) {
	var s string
	return s, e.db.QueryRow(query).Scan(&s)
}

// QueryRow runs a query expected to return exactly one row and scans its
// columns into dest. Used for admin calls such as quack_serve, which returns
// (listen_uri, url, auth_token).
func (e *Engine) QueryRow(query string, dest ...any) error {
	return e.db.QueryRow(query).Scan(dest...)
}

// RequireDuckDB returns an error if the embedded DuckDB engine is older than
// the given version. Features like Quack need >= v1.5.3; calling this up front
// turns an obscure runtime failure into a clear, actionable message.
func (e *Engine) RequireDuckDB(major, minor, patch int) error {
	raw, err := e.ScalarString("SELECT version()")
	if err != nil {
		return fmt.Errorf("could not read DuckDB version: %w", err)
	}
	maj, min, pat, err := parseDuckDBVersion(raw)
	if err != nil {
		return fmt.Errorf("could not parse DuckDB version %q: %w", raw, err)
	}
	if maj < major || (maj == major && (min < minor || (min == minor && pat < patch))) {
		return fmt.Errorf(
			"DuckDB %s is too old; this feature needs >= v%d.%d.%d. Rebuild against a newer "+
				"go-duckdb: go get github.com/marcboeker/go-duckdb/v2@latest && go mod tidy",
			raw, major, minor, patch,
		)
	}
	return nil
}

func parseDuckDBVersion(s string) (maj, min, pat int, err error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+ "); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) < 3 {
		return 0, 0, 0, fmt.Errorf("expected major.minor.patch")
	}
	if maj, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, 0, err
	}
	if min, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, 0, err
	}
	if pat, err = strconv.Atoi(parts[2]); err != nil {
		return 0, 0, 0, err
	}
	return maj, min, pat, nil
}

var validParquetCompression = map[string]bool{
	"snappy": true, "zstd": true, "gzip": true, "lz4": true, "uncompressed": true,
}

// ExportParquet writes selectSQL to a Parquet file. The path is quoted as a
// literal (so quotes/spaces are safe) and compression is checked against an
// allowlist so it cannot be used to inject SQL.
func (e *Engine) ExportParquet(selectSQL, outputPath, compression string) error {
	if compression == "" {
		compression = "snappy"
	}
	if !validParquetCompression[compression] {
		return fmt.Errorf("unsupported parquet compression %q (valid: snappy, zstd, gzip, lz4, uncompressed)", compression)
	}
	return e.Exec(fmt.Sprintf(
		"COPY (%s) TO %s (FORMAT PARQUET, COMPRESSION %s)",
		selectSQL, sqlutil.QuoteLiteral(outputPath), compression,
	))
}

// ExportCSV writes selectSQL to a CSV file with a header row.
func (e *Engine) ExportCSV(selectSQL, outputPath string) error {
	return e.Exec(fmt.Sprintf(
		"COPY (%s) TO %s (FORMAT CSV, HEADER true)",
		selectSQL, sqlutil.QuoteLiteral(outputPath),
	))
}

func (e *Engine) Close() error { return e.db.Close() }
