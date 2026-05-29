package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
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
	DSN      string            `yaml:"dsn"`      // postgres
	Table    string            `yaml:"table"`    // postgres, motherduck, quack
	Database string            `yaml:"database"` // quack
	URL      string            `yaml:"url"`      // quack
	Token    string            `yaml:"token"`    // quack — must use ${VAR} syntax
	Options  map[string]string `yaml:"options"`
}

type Output struct {
	Type        string `yaml:"type"`
	Path        string `yaml:"path"`
	Database    string `yaml:"database"`
	Table       string `yaml:"table"`
	Compression string `yaml:"compression"`
	Mode        string `yaml:"mode"`
	URL         string `yaml:"url"`   // quack
	Token       string `yaml:"token"` // quack — must use ${VAR} syntax
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
		if seen[s.Name] {
			return fmt.Errorf("duplicate source name %q — each source name must be unique", s.Name)
		}
		seen[s.Name] = true
		validTypes := map[string]bool{
			"csv": true, "json": true, "parquet": true,
			"postgres": true, "motherduck": true, "quack": true,
		}
		if !validTypes[s.Type] {
			return fmt.Errorf("source %q has unknown type %q (valid: csv, json, parquet, postgres, motherduck, quack)", s.Name, s.Type)
		}
		if (s.Type == "csv" || s.Type == "json" || s.Type == "parquet") && s.Path == "" {
			return fmt.Errorf("source %q (type=%s) requires a 'path'", s.Name, s.Type)
		}
		if s.Type == "quack" {
			if s.URL == "" {
				return fmt.Errorf("source %q: quack requires a 'url' field", s.Name)
			}
			if s.Table == "" {
				return fmt.Errorf("source %q: quack requires a 'table' field", s.Name)
			}
			if err := validateToken(fmt.Sprintf("source %q token", s.Name), s.Token); err != nil {
				return err
			}
		}
	}
	validOutputs := map[string]bool{"parquet": true, "csv": true, "motherduck": true, "quack": true}
	if !validOutputs[p.Output.Type] {
		return fmt.Errorf("output type %q is not supported (valid: parquet, csv, motherduck, quack)", p.Output.Type)
	}
	if p.Output.Type == "quack" {
		if p.Output.URL == "" {
			return fmt.Errorf("output: quack requires a 'url' field")
		}
		if p.Output.Table == "" {
			return fmt.Errorf("output: quack requires a 'table' field")
		}
		if err := validateToken("output.token", p.Output.Token); err != nil {
			return err
		}
	}
	return nil
}

// validateToken rejects bare literal tokens — they must use ${VAR} interpolation.
func validateToken(field, token string) error {
	if token == "" {
		return nil
	}
	if !strings.Contains(token, "${") {
		return fmt.Errorf("%s must reference an environment variable, e.g. token: ${QUACK_TOKEN} — never put secrets in YAML files", field)
	}
	return nil
}
