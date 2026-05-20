package loader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mehrabr/waddler/internal/config"
	"github.com/mehrabr/waddler/internal/engine"
)

func Write(eng *engine.Engine, p *config.Pipeline) (string, error) {
	out := p.Output
	switch out.Type {
	case "parquet":
		if err := os.MkdirAll(filepath.Dir(out.Path), 0o755); err != nil {
			return "", fmt.Errorf("loader: create output dir: %w", err)
		}
		return out.Path, eng.ExportParquet(p.Transform, out.Path, out.Compression)
	case "csv":
		if err := os.MkdirAll(filepath.Dir(out.Path), 0o755); err != nil {
			return "", fmt.Errorf("loader: create output dir: %w", err)
		}
		return out.Path, eng.ExportCSV(p.Transform, out.Path)
	case "motherduck":
		return writeMotherDuck(eng, p)
	default:
		return "", fmt.Errorf("unknown output type %q", out.Type)
	}
}

// writeMotherDuck writes pipeline output to a MotherDuck cloud table.
// The ATTACH alias is "md_out_<table>" — distinct from source aliases
// ("md_<name>") so read-from and write-to MotherDuck can coexist.
func writeMotherDuck(eng *engine.Engine, p *config.Pipeline) (string, error) {
	token := os.Getenv("MOTHERDUCK_TOKEN")
	if token == "" {
		return "", fmt.Errorf("motherduck output requires MOTHERDUCK_TOKEN env var")
	}
	out := p.Output
	dbName := out.Database
	if dbName == "" {
		dbName = "my_db"
	}
	if out.Table == "" {
		return "", fmt.Errorf("motherduck output requires a 'table' field")
	}
	alias := "md_out_" + out.Table
	attachSQL := fmt.Sprintf("ATTACH 'md:%s?motherduck_token=%s' AS %s", dbName, token, alias)
	if err := eng.Exec(attachSQL); err != nil {
		return "", fmt.Errorf("loader: attach motherduck: %w", err)
	}
	var stmt string
	if out.Mode == "append" {
		ensureSQL := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s.%s AS (%s) WHERE false",
			alias, out.Table, p.Transform,
		)
		if err := eng.Exec(ensureSQL); err != nil {
			return "", fmt.Errorf("loader: motherduck ensure table: %w", err)
		}
		stmt = fmt.Sprintf("INSERT INTO %s.%s BY NAME (%s)", alias, out.Table, p.Transform)
	} else {
		stmt = fmt.Sprintf("CREATE OR REPLACE TABLE %s.%s AS (%s)", alias, out.Table, p.Transform)
	}
	location := fmt.Sprintf("motherduck:%s.%s", dbName, out.Table)
	return location, eng.Exec(stmt)
}
