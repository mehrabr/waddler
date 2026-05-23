package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mehrabr/waddler/internal/config"
	"github.com/mehrabr/waddler/internal/runner"
)

func testdataPath(file string) string {
	abs, _ := filepath.Abs(filepath.Join("..", "..", "testdata", file))
	return abs
}

func TestRun_DonorReportToParquet(t *testing.T) {
	tmpOut := filepath.Join(t.TempDir(), "out.parquet")
	p := &config.Pipeline{
		Name: "donor-test",
		Sources: []config.Source{
			{Name: "donations", Type: "csv", Path: testdataPath("donations.csv")},
			{Name: "donors", Type: "csv", Path: testdataPath("donors.csv")},
		},
		Transform: `SELECT d.donor_id, dn.name, SUM(d.amount) AS total
			FROM donations d JOIN donors dn USING (donor_id)
			WHERE d.amount > 0 GROUP BY ALL`,
		Output: config.Output{Type: "parquet", Path: tmpOut},
	}
	result, err := runner.Run(p)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsOut != 2 {
		t.Errorf("expected 2 rows, got %d", result.RowsOut)
	}
	if _, err := os.Stat(tmpOut); err != nil {
		t.Errorf("output file not found: %v", err)
	}
}

func TestRun_OutputCSV(t *testing.T) {
	tmpOut := filepath.Join(t.TempDir(), "out.csv")
	p := &config.Pipeline{
		Name:      "csv-output-test",
		Sources:   []config.Source{{Name: "donations", Type: "csv", Path: testdataPath("donations.csv")}},
		Transform: "SELECT donor_id, amount FROM donations ORDER BY amount DESC",
		Output:    config.Output{Type: "csv", Path: tmpOut},
	}
	result, err := runner.Run(p)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsOut != 4 {
		t.Errorf("expected 4 rows, got %d", result.RowsOut)
	}
}

func TestRun_MissingSourceFile(t *testing.T) {
	p := &config.Pipeline{
		Name:      "missing-file",
		Sources:   []config.Source{{Name: "ghost", Type: "csv", Path: "/nonexistent/ghost.csv"}},
		Transform: "SELECT * FROM ghost",
		Output:    config.Output{Type: "csv", Path: filepath.Join(t.TempDir(), "out.csv")},
	}
	if _, err := runner.Run(p); err == nil {
		t.Error("expected error for missing source file")
	}
}

func TestRun_ValidationPass(t *testing.T) {
	min := int64(1)
	max := int64(10)
	p := &config.Pipeline{
		Name:      "validation-pass",
		Sources:   []config.Source{{Name: "donations", Type: "csv", Path: testdataPath("donations.csv")}},
		Transform: "SELECT * FROM donations WHERE amount > 0",
		Validate: []config.Assertion{
			{Name: "at least one row", SQL: "SELECT COUNT(*) FROM ({transform})", ExpectMin: &min},
			{Name: "not too many rows", SQL: "SELECT COUNT(*) FROM ({transform})", ExpectMax: &max},
		},
		Output: config.Output{Type: "csv", Path: filepath.Join(t.TempDir(), "out.csv")},
	}
	if _, err := runner.Run(p); err != nil {
		t.Fatalf("expected pass, got: %v", err)
	}
}

func TestRun_ValidationFail(t *testing.T) {
	zero := int64(0)
	p := &config.Pipeline{
		Name:      "validation-fail",
		Sources:   []config.Source{{Name: "donations", Type: "csv", Path: testdataPath("donations.csv")}},
		Transform: "SELECT * FROM donations",
		Validate:  []config.Assertion{{Name: "no rows", SQL: "SELECT COUNT(*) FROM ({transform})", Expect: &zero}},
		Output:    config.Output{Type: "csv", Path: filepath.Join(t.TempDir(), "out.csv")},
	}
	if _, err := runner.Run(p); err == nil {
		t.Error("expected validation failure")
	}
}
