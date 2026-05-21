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
	Name    string            `yaml:"name"`
	Type    string            `yaml:"type"`
	Path    string            `yaml:"path"`
	DSN     string            `yaml:"dsn"`
	Table   string            `yaml:"table"`
	Options map[string]string `yaml:"options"`
}

type Output struct {
	Type        string `yaml:"type"`
	Path        string `yaml:"path"`
	Database    string `yaml:"database"`
	Table       string `yaml:"table"`
	Compression string `yaml:"compression"`
	Mode        string `yaml:"mode"`
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
		validTypes := map[string]bool{"csv": true, "json": true, "parquet": true, "postgres": true, "motherduck": true}
		if !validTypes[s.Type] {
			return fmt.Errorf("source %q has unknown type %q (valid: csv, json, parquet, postgres, motherduck)", s.Name, s.Type)
		}
		if (s.Type == "csv" || s.Type == "json" || s.Type == "parquet") && s.Path == "" {
			return fmt.Errorf("source %q (type=%s) requires a 'path'", s.Name, s.Type)
		}
	}
	validOutputs := map[string]bool{"parquet": true, "csv": true, "motherduck": true}
	if !validOutputs[p.Output.Type] {
		return fmt.Errorf("output type %q is not supported (valid: parquet, csv, motherduck)", p.Output.Type)
	}
	return nil
}
