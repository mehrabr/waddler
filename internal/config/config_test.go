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

func TestLoad_QuackSource_Valid(t *testing.T) {
	path := writeTmp(t, `
name: quack-src
sources:
  - name: remote
    type: quack
    url: quack:localhost:9494
    token: ${QUACK_TOKEN}
    table: donor_report
transform: SELECT * FROM remote
output:
  type: csv
  path: /tmp/out.csv
`)
	if _, err := config.Load(path); err != nil {
		t.Fatalf("expected valid quack source, got: %v", err)
	}
}

func TestLoad_QuackSource_MissingURL(t *testing.T) {
	path := writeTmp(t, `
name: quack-no-url
sources:
  - name: remote
    type: quack
    table: donor_report
transform: SELECT * FROM remote
output:
  type: csv
  path: /tmp/out.csv
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for missing url")
	}
}

func TestLoad_QuackSource_MissingTable(t *testing.T) {
	path := writeTmp(t, `
name: quack-no-table
sources:
  - name: remote
    type: quack
    url: quack:localhost:9494
transform: SELECT * FROM remote
output:
  type: csv
  path: /tmp/out.csv
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for missing table")
	}
}

func TestLoad_QuackSource_BareToken(t *testing.T) {
	path := writeTmp(t, `
name: quack-bare-token
sources:
  - name: remote
    type: quack
    url: quack:localhost:9494
    token: supersecrettoken
    table: donor_report
transform: SELECT * FROM remote
output:
  type: csv
  path: /tmp/out.csv
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for bare literal token")
	}
}

func TestLoad_QuackOutput_Valid(t *testing.T) {
	path := writeTmp(t, `
name: quack-out
sources:
  - name: s1
    type: csv
    path: /tmp/x.csv
transform: SELECT * FROM s1
output:
  type: quack
  url: quack:localhost:9494
  token: ${QUACK_TOKEN}
  table: donor_report
`)
	if _, err := config.Load(path); err != nil {
		t.Fatalf("expected valid quack output, got: %v", err)
	}
}

func TestLoad_QuackOutput_BareToken(t *testing.T) {
	path := writeTmp(t, `
name: quack-out-bare
sources:
  - name: s1
    type: csv
    path: /tmp/x.csv
transform: SELECT * FROM s1
output:
  type: quack
  url: quack:localhost:9494
  token: literaltoken
  table: donor_report
`)
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for bare literal output token")
	}
}

func TestLoad_WithSchedule(t *testing.T) {
	path := writeTmp(t, `
name: scheduled
sources:
  - name: s1
    type: csv
    path: /tmp/x.csv
transform: SELECT * FROM s1
schedule: "0 6 * * *"
output:
  type: csv
  path: /tmp/out.csv
`)
	p, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !p.HasSchedule() {
		t.Error("expected HasSchedule() to return true")
	}
}

func TestLoad_WithValidation(t *testing.T) {
	path := writeTmp(t, `
name: validated
sources:
  - name: s1
    type: csv
    path: /tmp/x.csv
transform: SELECT * FROM s1
validate:
  - name: row count positive
    sql: SELECT COUNT(*) FROM ({transform})
    expect_min: 1
output:
  type: csv
  path: /tmp/out.csv
`)
	p, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Validate) != 1 {
		t.Errorf("expected 1 validation rule, got %d", len(p.Validate))
	}
}
