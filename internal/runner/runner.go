package runner

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mehrabr/waddler/internal/config"
	"github.com/mehrabr/waddler/internal/engine"
	"github.com/mehrabr/waddler/internal/loader"
	"github.com/mehrabr/waddler/internal/source"
)

type Result struct {
	Pipeline   string
	OutputPath string
	RowsOut    int64
	Duration   time.Duration
}

func (r *Result) String() string {
	return fmt.Sprintf("%s: %d rows → %s in %s",
		r.Pipeline, r.RowsOut, r.OutputPath,
		r.Duration.Round(time.Millisecond),
	)
}

func Run(p *config.Pipeline) (*Result, error) {
	start := time.Now()
	log := slog.Default()

	log.Info("starting pipeline", "name", p.Name)

	eng, err := engine.New()
	if err != nil {
		return nil, fmt.Errorf("runner: open engine: %w", err)
	}
	defer eng.Close()

	for _, s := range p.Sources {
		log.Info("registering source", "name", s.Name, "type", s.Type)
		if err := source.Register(eng, s); err != nil {
			return nil, fmt.Errorf("runner: source %q: %w", s.Name, err)
		}
	}

	log.Info("running transform")
	rowCount, err := eng.RowCount(p.Transform)
	if err != nil {
		return nil, fmt.Errorf(
			"runner: transform error: %w\n\nCheck your SQL in the 'transform' field", err,
		)
	}

	if len(p.Validate) > 0 {
		log.Info("running validation rules", "count", len(p.Validate))
		if err := runAssertions(eng, p); err != nil {
			return nil, fmt.Errorf("runner: validation failed: %w", err)
		}
		log.Info("all validation rules passed")
	}

	outputPath, err := loader.Write(eng, p)
	if err != nil {
		return nil, fmt.Errorf("runner: output: %w", err)
	}

	result := &Result{
		Pipeline:   p.Name,
		OutputPath: outputPath,
		RowsOut:    rowCount,
		Duration:   time.Since(start),
	}
	log.Info("pipeline complete",
		"rows", rowCount, "output", outputPath,
		"duration", result.Duration.Round(time.Millisecond),
	)
	return result, nil
}

// runAssertions runs each validate[] rule. All failures are collected
// before returning so the user sees every problem in one shot.
func runAssertions(eng *engine.Engine, p *config.Pipeline) error {
	var errs []string
	for _, a := range p.Validate {
		q := strings.ReplaceAll(a.SQL, "{transform}", p.Transform)
		n, err := eng.ScalarInt64(q)
		if err != nil {
			return fmt.Errorf("assertion %q: %w", a.Name, err)
		}
		if a.Expect != nil && n != *a.Expect {
			errs = append(errs, fmt.Sprintf("  ✗ %q: expected %d, got %d", a.Name, *a.Expect, n))
		}
		if a.ExpectMin != nil && n < *a.ExpectMin {
			errs = append(errs, fmt.Sprintf("  ✗ %q: expected >= %d, got %d", a.Name, *a.ExpectMin, n))
		}
		if a.ExpectMax != nil && n > *a.ExpectMax {
			errs = append(errs, fmt.Sprintf("  ✗ %q: expected <= %d, got %d", a.Name, *a.ExpectMax, n))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("data quality checks failed:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}
