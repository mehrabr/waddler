package loader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mehrabr/waddler/internal/config"
	"github.com/mehrabr/waddler/internal/engine"
)

func Write(eng *engine.Engine, p *config.Pipeline) (string, error) {
	out := p.Output
	switch out.Type {
	case "parquet":
		if err := os.MkdirAll(filepath.Dir(out.Path), 0o755); err != nil {
			return "", fmt.Errorf("loader: create output dir: %w", err)
		}
		return out.Path, eng.ExportParquet(p.Transform, out.Path, out.Compression)
	case "csv":
		if err := os.MkdirAll(filepath.Dir(out.Path), 0o755); err != nil {
			return "", fmt.Errorf("loader: create output dir: %w", err)
		}
		return out.Path, eng.ExportCSV(p.Transform, out.Path)
	case "motherduck":
		return writeMotherDuck(eng, p)
	default:
		return "", fmt.Errorf("unknown output type %q", out.Type)
	}
}

func writeMotherDuck(_ *engine.Engine, _ *config.Pipeline) (string, error) {
	return "", fmt.Errorf("motherduck output not yet implemented")
}
