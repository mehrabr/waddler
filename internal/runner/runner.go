package runner

import (
	"fmt"
	"log/slog"
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
