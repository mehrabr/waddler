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
	case "quack":
		return registerQuack(eng, s)
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

// registerQuack reads a table from a remote waddler relay or any Quack-compatible server.
// The ATTACH alias is "quack_<source.name>" to avoid collisions with other sources.
func registerQuack(eng *engine.Engine, s config.Source) error {
	token, err := expandToken(s.Token)
	if err != nil {
		return fmt.Errorf("source %q: %w", s.Name, err)
	}
	for _, stmt := range []string{"INSTALL quack", "LOAD quack"} {
		if err := eng.Exec(stmt); err != nil {
			return fmt.Errorf("source %q: %w", s.Name, err)
		}
	}
	alias := "quack_" + s.Name
	attachSQL := fmt.Sprintf("ATTACH '%s?token=%s' AS %s (TYPE quack)", s.URL, token, alias)
	if err := eng.Exec(attachSQL); err != nil {
		return fmt.Errorf("source %q: quack attach: %w", s.Name, err)
	}
	var selectSQL string
	if s.Database != "" {
		selectSQL = fmt.Sprintf("SELECT * FROM %s.%s.%s", alias, s.Database, s.Table)
	} else {
		selectSQL = fmt.Sprintf("SELECT * FROM %s.%s", alias, s.Table)
	}
	return eng.CreateView(s.Name, selectSQL)
}

// registerMotherDuck reads from a MotherDuck cloud table.
// The ATTACH alias is "md_<source.name>" so two MotherDuck sources
// in the same pipeline get distinct aliases.
func registerMotherDuck(eng *engine.Engine, s config.Source) error {
	token := s.Options["token"]
	if token == "" {
		token = os.Getenv("MOTHERDUCK_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("source %q: set MOTHERDUCK_TOKEN or options.token", s.Name)
	}
	if s.Table == "" {
		return fmt.Errorf("source %q: motherduck requires a 'table' field", s.Name)
	}
	dbName := s.Options["database"]
	if dbName == "" {
		dbName = "my_db"
	}
	alias := "md_" + s.Name
	attachSQL := fmt.Sprintf("ATTACH 'md:%s?motherduck_token=%s' AS %s", dbName, token, alias)
	if err := eng.Exec(attachSQL); err != nil {
		return fmt.Errorf("source %q: attach motherduck: %w", s.Name, err)
	}
	return eng.CreateView(s.Name, fmt.Sprintf("SELECT * FROM %s.%s", alias, s.Table))
}

// expandToken resolves ${VAR} references in a token string.
// Returns an error if a referenced variable is not set.
func expandToken(token string) (string, error) {
	var missing string
	result := os.Expand(token, func(key string) string {
		val := os.Getenv(key)
		if val == "" && missing == "" {
			missing = key
		}
		return val
	})
	if missing != "" {
		return "", fmt.Errorf("environment variable %q is not set", missing)
	}
	return result, nil
}
