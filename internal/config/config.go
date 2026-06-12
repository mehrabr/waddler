package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mehrabr/waddler/internal/sqlutil"
)

type Pipeline struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Sources     []Source    `yaml:"sources"`
	Transform   string      `yaml:"transform"`
	Validate    []Assertion `yaml:"validate"`
	Output      Output      `yaml:"output"`
	Schedule    string      `yaml:"schedule"`
}

type Source struct {
	Name     string            `yaml:"name"`
	Type     string            `yaml:"type"`
	Path     string            `yaml:"path"`     // csv, json, parquet
	DSN      string            `yaml:"dsn"`      // postgres (may use ${VAR})
	URL      string            `yaml:"url"`      // quack (e.g. quack:host:port)
	Table    string            `yaml:"table"`    // postgres, motherduck, quack
	Database string            `yaml:"database"` // motherduck
	Token    string            `yaml:"token"`    // motherduck, quack — must use ${VAR}
	Options  map[string]string `yaml:"options"`  // e.g. postgres schema
}

type Output struct {
	Type        string `yaml:"type"`
	Path        string `yaml:"path"`        // parquet, csv
	URL         string `yaml:"url"`         // quack (e.g. quack:host:port)
	Database    string `yaml:"database"`    // motherduck
	Table       string `yaml:"table"`       // motherduck, quack
	Compression string `yaml:"compression"` // parquet
	Mode        string `yaml:"mode"`        // motherduck/quack: replace (default) | append
	Token       string `yaml:"token"`       // motherduck, quack — must use ${VAR}
}

// Assertion is one data-quality rule in the validate[] block.
type Assertion struct {
	Name      string `yaml:"name"`
	SQL       string `yaml:"sql"`
	Expect    *int64 `yaml:"expect"`
	ExpectMin *int64 `yaml:"expect_min"`
	ExpectMax *int64 `yaml:"expect_max"`
}

// HasSchedule reports whether the pipeline has a cron schedule configured.
func (p *Pipeline) HasSchedule() bool {
	return strings.TrimSpace(p.Schedule) != ""
}

func Load(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read pipeline file %q: %w", path, err)
	}
	var p Pipeline
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid YAML in %q: %w", path, err)
	}
	return &p, validate(&p)
}

var validSourceTypes = map[string]bool{
	"csv": true, "json": true, "parquet": true, "postgres": true, "motherduck": true, "quack": true,
}

var validOutputTypes = map[string]bool{
	"parquet": true, "csv": true, "motherduck": true, "quack": true,
}

func validate(p *Pipeline) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("pipeline is missing a 'name' field")
	}
	if len(p.Sources) == 0 {
		return fmt.Errorf("pipeline %q has no sources defined", p.Name)
	}
	if strings.TrimSpace(p.Transform) == "" {
		return fmt.Errorf("pipeline %q has no transform SQL", p.Name)
	}

	seen := make(map[string]bool)
	for i, s := range p.Sources {
		if s.Name == "" {
			return fmt.Errorf("source #%d is missing a 'name'", i+1)
		}
		if !sqlutil.IsSimpleIdent(s.Name) {
			return fmt.Errorf("source name %q must be a valid identifier (letters, digits, underscores; not starting with a digit) so you can reference it in transform SQL", s.Name)
		}
		if seen[s.Name] {
			return fmt.Errorf("duplicate source name %q — each source name must be unique", s.Name)
		}
		seen[s.Name] = true

		if !validSourceTypes[s.Type] {
			return fmt.Errorf("source %q has unknown type %q (valid: csv, json, parquet, postgres, motherduck, quack)", s.Name, s.Type)
		}
		switch s.Type {
		case "csv", "json", "parquet":
			if s.Path == "" {
				return fmt.Errorf("source %q (type=%s) requires a 'path'", s.Name, s.Type)
			}
		case "postgres":
			if s.DSN == "" {
				return fmt.Errorf("source %q (type=postgres) requires a 'dsn'", s.Name)
			}
			if s.Table == "" {
				return fmt.Errorf("source %q (type=postgres) requires a 'table'", s.Name)
			}
		case "motherduck":
			if s.Table == "" {
				return fmt.Errorf("source %q (type=motherduck) requires a 'table'", s.Name)
			}
			if err := requireEnvToken(fmt.Sprintf("source %q token", s.Name), s.Token); err != nil {
				return err
			}
		case "quack":
			if s.Table == "" {
				return fmt.Errorf("source %q (type=quack) requires a 'table'", s.Name)
			}
			if err := requireQuackURL(fmt.Sprintf("source %q", s.Name), s.URL); err != nil {
				return err
			}
			if err := requireEnvToken(fmt.Sprintf("source %q token", s.Name), s.Token); err != nil {
				return err
			}
		}
	}

	if !validOutputTypes[p.Output.Type] {
		return fmt.Errorf("output type %q is not supported (valid: parquet, csv, motherduck, quack)", p.Output.Type)
	}
	switch p.Output.Type {
	case "parquet", "csv":
		if p.Output.Path == "" {
			return fmt.Errorf("output (type=%s) requires a 'path'", p.Output.Type)
		}
	case "motherduck":
		if p.Output.Table == "" {
			return fmt.Errorf("output (type=motherduck) requires a 'table'")
		}
		if err := requireMode(p.Output.Mode); err != nil {
			return err
		}
		if err := requireEnvToken("output token", p.Output.Token); err != nil {
			return err
		}
	case "quack":
		if p.Output.Table == "" {
			return fmt.Errorf("output (type=quack) requires a 'table'")
		}
		if err := requireQuackURL("output", p.Output.URL); err != nil {
			return err
		}
		if err := requireMode(p.Output.Mode); err != nil {
			return err
		}
		if err := requireEnvToken("output token", p.Output.Token); err != nil {
			return err
		}
	}
	return nil
}

func requireMode(mode string) error {
	if mode != "" && mode != "replace" && mode != "append" {
		return fmt.Errorf("output mode %q is invalid (valid: replace, append)", mode)
	}
	return nil
}

// requireQuackURL checks that a quack endpoint URL is present and well-formed.
func requireQuackURL(field, url string) error {
	if url == "" {
		return fmt.Errorf("%s (type=quack) requires a 'url' (e.g. quack:hub.example.com:9494)", field)
	}
	if !strings.HasPrefix(url, "quack:") {
		return fmt.Errorf("%s url must start with 'quack:' (got %q)", field, url)
	}
	return nil
}

// requireEnvToken rejects bare literal tokens in YAML — they must reference an
// environment variable via ${VAR}. An empty token is allowed (the token then
// comes from an environment variable resolved at run time).
func requireEnvToken(field, token string) error {
	if token == "" {
		return nil
	}
	if !strings.Contains(token, "${") {
		return fmt.Errorf("%s must reference an environment variable, e.g. token: ${WADDLER_QUACK_TOKEN} — never put secrets in YAML files", field)
	}
	return nil
}
