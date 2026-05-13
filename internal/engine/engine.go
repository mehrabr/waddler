package engine

import (
	"database/sql"
	"fmt"

	_ "github.com/marcboeker/go-duckdb"
)

type Engine struct{ db *sql.DB }

func New() (*Engine, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("engine: open duckdb: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("engine: ping: %w", err)
	}
	return &Engine{db: db}, nil
}

func (e *Engine) Exec(query string, args ...any) error {
	if _, err := e.db.Exec(query, args...); err != nil {
		return fmt.Errorf("engine exec: %w\nSQL: %s", err, query)
	}
	return nil
}

func (e *Engine) CreateView(name, selectSQL string) error {
	return e.Exec(fmt.Sprintf("CREATE OR REPLACE VIEW %s AS %s", name, selectSQL))
}

func (e *Engine) ExportParquet(transformSQL, outputPath, compression string) error {
	if compression == "" {
		compression = "snappy"
	}
	return e.Exec(fmt.Sprintf(
		"COPY (%s) TO '%s' (FORMAT PARQUET, COMPRESSION %s)",
		transformSQL, outputPath, compression,
	))
}

func (e *Engine) ExportCSV(transformSQL, outputPath string) error {
	return e.Exec(fmt.Sprintf(
		"COPY (%s) TO '%s' (FORMAT CSV, HEADER true)",
		transformSQL, outputPath,
	))
}

func (e *Engine) RowCount(transformSQL string) (int64, error) {
	var n int64
	return n, e.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM (%s)", transformSQL)).Scan(&n)
}

func (e *Engine) ScalarInt64(query string) (int64, error) {
	var n int64
	return n, e.db.QueryRow(query).Scan(&n)
}

func (e *Engine) Close() error { return e.db.Close() }
