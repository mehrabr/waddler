package source

import (
	"fmt"
	"os"

	"github.com/mehrabr/waddler/internal/config"
	"github.com/mehrabr/waddler/internal/engine"
)

// Register attaches a source to the DuckDB engine as a named view.
func Register(eng *engine.Engine, s config.Source) error {
	switch s.Type {
	case "csv":
		return registerFile(eng, s, "read_csv_auto")
	case "json":
		return registerFile(eng, s, "read_json_auto")
	case "parquet":
		return registerFile(eng, s, "read_parquet")
	case "postgres":
		return registerPostgres(eng, s)
	case "motherduck":
		return registerMotherDuck(eng, s)
	default:
		return fmt.Errorf("source: unknown type %q", s.Type)
	}
}

func registerFile(eng *engine.Engine, s config.Source, fn string) error {
	if _, err := os.Stat(s.Path); os.IsNotExist(err) {
		return fmt.Errorf("source %q: file not found: %s", s.Name, s.Path)
	}
	sql := fmt.Sprintf("SELECT * FROM %s('%s')", fn, s.Path)
	if err := eng.CreateView(s.Name, sql); err != nil {
		return fmt.Errorf("source %q: %w", s.Name, err)
	}
	return nil
}

// postgres and motherduck stubs — implemented in upcoming commits
func registerPostgres(eng *engine.Engine, s config.Source) error {
	return fmt.Errorf("source %q: postgres not yet implemented", s.Name)
}
func registerMotherDuck(eng *engine.Engine, s config.Source) error {
	return fmt.Errorf("source %q: motherduck not yet implemented", s.Name)
}
