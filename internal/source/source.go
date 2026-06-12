package source

import (
	"fmt"
	"os"

	"github.com/mehrabr/waddler/internal/config"
	"github.com/mehrabr/waddler/internal/engine"
	"github.com/mehrabr/waddler/internal/secret"
	"github.com/mehrabr/waddler/internal/sqlutil"
)

// Register makes a source available to the transform as a view named s.Name.
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
	// Path is quoted as a literal so spaces / quotes in the path are safe.
	sql := fmt.Sprintf("SELECT * FROM %s(%s)", fn, sqlutil.QuoteLiteral(s.Path))
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
	dsn, err := secret.Expand(s.DSN) // allow dsn: ${CRM_DSN}
	if err != nil {
		return fmt.Errorf("source %q: %w", s.Name, err)
	}
	for _, stmt := range []string{"INSTALL postgres", "LOAD postgres"} {
		if err := eng.Exec(stmt); err != nil {
			return fmt.Errorf("source %q: load postgres extension: %w", s.Name, err)
		}
	}
	schema := s.Options["schema"]
	if schema == "" {
		schema = "public"
	}
	sql := fmt.Sprintf(
		"SELECT * FROM postgres_scan(%s, %s, %s)",
		sqlutil.QuoteLiteral(dsn), sqlutil.QuoteLiteral(schema), sqlutil.QuoteLiteral(s.Table),
	)
	if err := eng.CreateView(s.Name, sql); err != nil {
		return fmt.Errorf("source %q: %w", s.Name, err)
	}
	return nil
}

// registerMotherDuck reads from a MotherDuck cloud table. The ATTACH alias is
// "md_<source.name>" so two MotherDuck sources in one pipeline get distinct
// aliases. The token comes from the source's token: ${VAR} field, or the
// MOTHERDUCK_TOKEN environment variable.
func registerMotherDuck(eng *engine.Engine, s config.Source) error {
	if s.Table == "" {
		return fmt.Errorf("source %q: motherduck requires a 'table' field", s.Name)
	}
	token, err := motherduckToken(s.Token)
	if err != nil {
		return fmt.Errorf("source %q: %w", s.Name, err)
	}
	dbName := s.Database
	if dbName == "" {
		dbName = "my_db"
	}
	alias := "md_" + s.Name
	conn := fmt.Sprintf("md:%s?motherduck_token=%s", dbName, token)
	attachSQL := fmt.Sprintf("ATTACH %s AS %s", sqlutil.QuoteLiteral(conn), sqlutil.QuoteIdent(alias))
	if err := eng.Exec(attachSQL); err != nil {
		return fmt.Errorf("source %q: attach motherduck (check token and database name): %w", s.Name, err)
	}
	selectSQL := fmt.Sprintf("SELECT * FROM %s.%s", sqlutil.QuoteIdent(alias), sqlutil.QuoteIdent(s.Table))
	return eng.CreateView(s.Name, selectSQL)
}

// registerQuack reads a table from a DuckDB hub over the native Quack protocol
// (a `waddler hub`, or any quack_serve endpoint). The ATTACH alias is
// "quack_<source.name>".
func registerQuack(eng *engine.Engine, s config.Source) error {
	if s.Table == "" {
		return fmt.Errorf("source %q: quack requires a 'table' field", s.Name)
	}
	if s.URL == "" {
		return fmt.Errorf("source %q: quack requires a 'url' field (e.g. quack:hub.example.com:9494)", s.Name)
	}
	if err := eng.RequireDuckDB(1, 5, 3); err != nil {
		return fmt.Errorf("source %q: %w", s.Name, err)
	}
	token, err := quackToken(s.Token)
	if err != nil {
		return fmt.Errorf("source %q: %w", s.Name, err)
	}
	for _, stmt := range []string{"INSTALL quack", "LOAD quack"} {
		if err := eng.Exec(stmt); err != nil {
			return fmt.Errorf("source %q: load quack extension: %w", s.Name, err)
		}
	}
	alias := "quack_" + s.Name
	attachSQL := fmt.Sprintf("ATTACH %s AS %s (TYPE quack, TOKEN %s)",
		sqlutil.QuoteLiteral(s.URL), sqlutil.QuoteIdent(alias), sqlutil.QuoteLiteral(token))
	if err := eng.Exec(attachSQL); err != nil {
		return fmt.Errorf("source %q: attach quack hub (check url and token): %w", s.Name, err)
	}
	selectSQL := fmt.Sprintf("SELECT * FROM %s.main.%s", sqlutil.QuoteIdent(alias), sqlutil.QuoteIdent(s.Table))
	return eng.CreateView(s.Name, selectSQL)
}

// motherduckToken resolves a token from the (optional) ${VAR} field, falling
// back to the MOTHERDUCK_TOKEN environment variable.
func motherduckToken(field string) (string, error) {
	if field != "" {
		return secret.Expand(field)
	}
	if tok := os.Getenv("MOTHERDUCK_TOKEN"); tok != "" {
		return tok, nil
	}
	return "", fmt.Errorf("set the MOTHERDUCK_TOKEN environment variable, or a token: ${VAR} field")
}

// quackToken resolves a token from the (optional) ${VAR} field, falling back to
// the WADDLER_QUACK_TOKEN environment variable.
func quackToken(field string) (string, error) {
	if field != "" {
		return secret.Expand(field)
	}
	if tok := os.Getenv("WADDLER_QUACK_TOKEN"); tok != "" {
		return tok, nil
	}
	return "", fmt.Errorf("set the WADDLER_QUACK_TOKEN environment variable, or a token: ${VAR} field")
}
