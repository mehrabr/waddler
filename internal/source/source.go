package source

import (
	"fmt"
	"os"

	"github.com/mehrabr/waddler/internal/config"
	"github.com/mehrabr/waddler/internal/engine"
)

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

func registerPostgres(eng *engine.Engine, s config.Source) error {
	if s.DSN == "" {
		return fmt.Errorf("source %q: postgres requires a 'dsn' field", s.Name)
	}
	if s.Table == "" {
		return fmt.Errorf("source %q: postgres requires a 'table' field", s.Name)
	}
	for _, stmt := range []string{"INSTALL postgres", "LOAD postgres"} {
		if err := eng.Exec(stmt); err != nil {
			return fmt.Errorf("source %q: %w", s.Name, err)
		}
	}
	schema := s.Options["schema"]
	if schema == "" {
		schema = "public"
	}
	sql := fmt.Sprintf("SELECT * FROM postgres_scan('%s', '%s', '%s')", s.DSN, schema, s.Table)
	return eng.CreateView(s.Name, sql)
}

// motherduck stub — implemented next
func registerMotherDuck(_ *engine.Engine, s config.Source) error {
	return fmt.Errorf("source %q: motherduck not yet implemented", s.Name)
}
