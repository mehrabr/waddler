package loader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mehrabr/waddler/internal/config"
	"github.com/mehrabr/waddler/internal/engine"
	"github.com/mehrabr/waddler/internal/secret"
	"github.com/mehrabr/waddler/internal/sqlutil"
)

// Write sends the already-materialized result (resultSQL, e.g.
// `SELECT * FROM "_waddler_result"`) to the configured output and returns a
// human-readable description of where it went.
func Write(eng *engine.Engine, p *config.Pipeline, resultSQL string) (string, error) {
	out := p.Output
	switch out.Type {
	case "parquet":
		if err := ensureDir(out.Path); err != nil {
			return "", err
		}
		return out.Path, eng.ExportParquet(resultSQL, out.Path, out.Compression)
	case "csv":
		if err := ensureDir(out.Path); err != nil {
			return "", err
		}
		return out.Path, eng.ExportCSV(resultSQL, out.Path)
	case "motherduck":
		return writeMotherDuck(eng, out, resultSQL)
	case "quack":
		return writeQuack(eng, out, resultSQL)
	default:
		return "", fmt.Errorf("unknown output type %q", out.Type)
	}
}

func ensureDir(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("loader: create output dir: %w", err)
	}
	return nil
}

// writeMotherDuck writes the result to a MotherDuck cloud table. The ATTACH
// alias is "md_out_<table>" so it never collides with a source alias.
func writeMotherDuck(eng *engine.Engine, out config.Output, resultSQL string) (string, error) {
	if out.Table == "" {
		return "", fmt.Errorf("motherduck output requires a 'table' field")
	}
	token, err := motherduckToken(out.Token)
	if err != nil {
		return "", fmt.Errorf("motherduck output: %w", err)
	}
	dbName := out.Database
	if dbName == "" {
		dbName = "my_db"
	}
	alias := "md_out_" + out.Table
	conn := fmt.Sprintf("md:%s?motherduck_token=%s", dbName, token)
	attachSQL := fmt.Sprintf("ATTACH %s AS %s", sqlutil.QuoteLiteral(conn), sqlutil.QuoteIdent(alias))
	if err := eng.Exec(attachSQL); err != nil {
		return "", fmt.Errorf("loader: attach motherduck (check token and database name): %w", err)
	}

	target := sqlutil.QuoteIdent(alias) + "." + sqlutil.QuoteIdent(out.Table)
	if err := writeTo(eng, target, out.Mode, resultSQL); err != nil {
		return "", fmt.Errorf("loader: motherduck: %w", err)
	}
	return fmt.Sprintf("motherduck:%s.%s", dbName, out.Table), nil
}

// writeQuack pushes the result to a DuckDB hub over the native Quack protocol
// (a `waddler hub`, or any quack_serve endpoint). This replaces the old
// hand-rolled HTTP relay: results are written with a real ATTACH + INSERT.
func writeQuack(eng *engine.Engine, out config.Output, resultSQL string) (string, error) {
	if err := eng.RequireDuckDB(1, 5, 3); err != nil {
		return "", fmt.Errorf("quack output: %w", err)
	}
	if out.Table == "" {
		return "", fmt.Errorf("quack output requires a 'table' field")
	}
	if out.URL == "" {
		return "", fmt.Errorf("quack output requires a 'url' field (e.g. quack:hub.example.com:9494)")
	}
	token, err := quackToken(out.Token)
	if err != nil {
		return "", fmt.Errorf("quack output: %w", err)
	}
	for _, stmt := range []string{"INSTALL quack", "LOAD quack"} {
		if err := eng.Exec(stmt); err != nil {
			return "", fmt.Errorf("loader: load quack extension: %w", err)
		}
	}
	alias := "quack_out"
	attachSQL := fmt.Sprintf("ATTACH %s AS %s (TYPE quack, TOKEN %s)",
		sqlutil.QuoteLiteral(out.URL), sqlutil.QuoteIdent(alias), sqlutil.QuoteLiteral(token))
	if err := eng.Exec(attachSQL); err != nil {
		return "", fmt.Errorf("loader: attach quack hub (check url and token): %w", err)
	}
	target := sqlutil.QuoteIdent(alias) + ".main." + sqlutil.QuoteIdent(out.Table)
	if err := writeTo(eng, target, out.Mode, resultSQL); err != nil {
		return "", fmt.Errorf("loader: quack: %w", err)
	}
	return fmt.Sprintf("%s/%s", out.URL, out.Table), nil
}

// writeTo writes resultSQL into an attached target table, in replace (default)
// or append mode. The result is wrapped in a subquery so a transform ending in
// ORDER BY / GROUP BY stays valid.
func writeTo(eng *engine.Engine, target, mode, resultSQL string) error {
	if mode == "append" {
		ensureSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s AS SELECT * FROM (%s) AS _src WHERE 1=0", target, resultSQL)
		if err := eng.Exec(ensureSQL); err != nil {
			return fmt.Errorf("ensure table: %w", err)
		}
		insertSQL := fmt.Sprintf("INSERT INTO %s BY NAME SELECT * FROM (%s) AS _src", target, resultSQL)
		if err := eng.Exec(insertSQL); err != nil {
			return fmt.Errorf("append: %w", err)
		}
		return nil
	}
	replaceSQL := fmt.Sprintf("CREATE OR REPLACE TABLE %s AS %s", target, resultSQL)
	if err := eng.Exec(replaceSQL); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func motherduckToken(field string) (string, error) {
	if field != "" {
		return secret.Expand(field)
	}
	if tok := os.Getenv("MOTHERDUCK_TOKEN"); tok != "" {
		return tok, nil
	}
	return "", fmt.Errorf("set the MOTHERDUCK_TOKEN environment variable, or a token: ${VAR} field")
}

func quackToken(field string) (string, error) {
	if field != "" {
		return secret.Expand(field)
	}
	if tok := os.Getenv("WADDLER_QUACK_TOKEN"); tok != "" {
		return tok, nil
	}
	return "", fmt.Errorf("set the WADDLER_QUACK_TOKEN environment variable, or a token: ${VAR} field")
}
