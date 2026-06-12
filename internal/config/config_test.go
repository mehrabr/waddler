package config_test

import (
	"os"
	"testing"

	"github.com/mehrabr/waddler/internal/config"
)

func writeTmp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "waddler-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func TestLoad_Valid(t *testing.T) {
	path := writeTmp(t, `
name: test
sources:
  - name: s1
    type: csv
    path: /tmp/fake.csv
transform: SELECT * FROM s1
output:
  type: parquet
  path: /tmp/out.parquet
`)
	p, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "test" {
		t.Errorf("got name %q", p.Name)
	}
}

func TestLoad_MissingName(t *testing.T) {
	path := writeTmp(t, `
sources:
  - name: s1
    type: csv
    path: x.csv
transform: SELECT 1
output:
  type: parquet
  path: out.parquet
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for missing name")
	}
}

func TestLoad_NoSources(t *testing.T) {
	path := writeTmp(t, `
name: nosrc
sources: []
transform: SELECT 1
output:
  type: parquet
  path: out.parquet
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for empty sources")
	}
}

func TestLoad_NoTransform(t *testing.T) {
	path := writeTmp(t, `
name: notransform
sources:
  - name: s1
    type: csv
    path: x.csv
output:
  type: parquet
  path: out.parquet
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for missing transform")
	}
}

func TestLoad_DuplicateSourceName(t *testing.T) {
	path := writeTmp(t, `
name: dup
sources:
  - name: s1
    type: csv
    path: a.csv
  - name: s1
    type: csv
    path: b.csv
transform: SELECT 1
output:
  type: parquet
  path: out.parquet
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for duplicate source name")
	}
}

func TestLoad_InvalidSourceName(t *testing.T) {
	path := writeTmp(t, `
name: badname
sources:
  - name: "bad name"
    type: csv
    path: a.csv
transform: SELECT 1
output:
  type: parquet
  path: out.parquet
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for source name that is not a valid identifier")
	}
}

func TestLoad_UnknownSourceType(t *testing.T) {
	path := writeTmp(t, `
name: badtype
sources:
  - name: s1
    type: excel
    path: x.xlsx
transform: SELECT 1
output:
  type: parquet
  path: out.parquet
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for unknown source type")
	}
}

func TestLoad_UnknownOutputType(t *testing.T) {
	path := writeTmp(t, `
name: badout
sources:
  - name: s1
    type: csv
    path: x.csv
transform: SELECT 1
output:
  type: avro
  path: out.avro
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for unknown output type")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	if _, err := config.Load("/no/such/file.yml"); err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoad_MotherDuckOutput_MissingTable(t *testing.T) {
	path := writeTmp(t, `
name: md-no-table
sources:
  - name: s1
    type: csv
    path: x.csv
transform: SELECT * FROM s1
output:
  type: motherduck
  database: analytics
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for motherduck output without a table")
	}
}

func TestLoad_MotherDuckOutput_LiteralTokenRejected(t *testing.T) {
	path := writeTmp(t, `
name: md-literal-token
sources:
  - name: s1
    type: csv
    path: x.csv
transform: SELECT * FROM s1
output:
  type: motherduck
  table: t
  token: "literal-secret-token"
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error: literal tokens in YAML must use ${VAR}")
	}
}

func TestLoad_MotherDuckOutput_EnvTokenAllowed(t *testing.T) {
	path := writeTmp(t, `
name: md-env-token
sources:
  - name: s1
    type: csv
    path: x.csv
transform: SELECT * FROM s1
output:
  type: motherduck
  table: t
  token: ${MOTHERDUCK_TOKEN}
`)
	if _, err := config.Load(path); err != nil {
		t.Fatalf("expected valid config with ${VAR} token, got: %v", err)
	}
}

func TestLoad_InvalidMode(t *testing.T) {
	path := writeTmp(t, `
name: md-bad-mode
sources:
  - name: s1
    type: csv
    path: x.csv
transform: SELECT * FROM s1
output:
  type: motherduck
  table: t
  mode: upsert
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for invalid output mode")
	}
}

func TestLoad_QuackOutput_Valid(t *testing.T) {
	path := writeTmp(t, `
name: quack-out
sources:
  - name: s1
    type: csv
    path: x.csv
transform: SELECT * FROM s1
output:
  type: quack
  url: quack:hub.example.com:9494
  table: results
  token: ${WADDLER_QUACK_TOKEN}
`)
	if _, err := config.Load(path); err != nil {
		t.Fatalf("expected valid quack output, got: %v", err)
	}
}

func TestLoad_QuackOutput_MissingURL(t *testing.T) {
	path := writeTmp(t, `
name: quack-no-url
sources:
  - name: s1
    type: csv
    path: x.csv
transform: SELECT * FROM s1
output:
  type: quack
  table: results
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for quack output without a url")
	}
}

func TestLoad_QuackOutput_BadURLScheme(t *testing.T) {
	path := writeTmp(t, `
name: quack-bad-url
sources:
  - name: s1
    type: csv
    path: x.csv
transform: SELECT * FROM s1
output:
  type: quack
  url: http://hub.example.com:9494
  table: results
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for quack url that does not start with quack:")
	}
}

func TestLoad_QuackSource_Valid(t *testing.T) {
	path := writeTmp(t, `
name: quack-src
sources:
  - name: remote
    type: quack
    url: quack:hub.example.com:9494
    table: results
    token: ${WADDLER_QUACK_TOKEN}
transform: SELECT * FROM remote
output:
  type: csv
  path: /tmp/out.csv
`)
	if _, err := config.Load(path); err != nil {
		t.Fatalf("expected valid quack source, got: %v", err)
	}
}
