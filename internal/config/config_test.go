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
