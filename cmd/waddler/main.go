package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/mehrabr/waddler/internal/config"
	"github.com/mehrabr/waddler/internal/runner"
)

// version is overridden at link time: -ldflags "-X main.version=v1.2.3"
var version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "waddler",
		Short:   "Zero-code ETL pipelines powered by DuckDB",
		Version: version,
	}
	root.AddCommand(cmdRun(), cmdValidate(), cmdSources())
	if err := root.Execute(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func cmdRun() *cobra.Command {
	return &cobra.Command{
		Use:   "run [pipeline.yml]",
		Short: "Execute a pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.Load(args[0])
			if err != nil {
				return fmt.Errorf("❌ %w", err)
			}
			result, err := runner.Run(p)
			if err != nil {
				return fmt.Errorf("❌ pipeline failed: %w", err)
			}
			fmt.Printf("\n✅ %s — %d rows → %s (%s)\n",
				result.Pipeline, result.RowsOut,
				result.OutputPath, result.Duration.Round(1),
			)
			return nil
		},
	}
}

func cmdValidate() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [pipeline.yml]",
		Short: "Check a pipeline file for errors without running it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := config.Load(args[0]); err != nil {
				fmt.Printf("❌ %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✅ pipeline config is valid")
			return nil
		},
	}
}

func cmdSources() *cobra.Command {
	return &cobra.Command{
		Use:   "sources",
		Short: "List supported source and output types",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Println(`Sources:
  csv        — local CSV file (path required)
  json       — local JSON file (path required)
  parquet    — local Parquet file (path required)
  postgres   — PostgreSQL table (dsn + table required)
  motherduck — MotherDuck cloud table (MOTHERDUCK_TOKEN env + table required)

Outputs:
  parquet    — local Parquet file (path required)
  csv        — local CSV file (path required)
  motherduck — MotherDuck cloud table (database + table required)`)
		},
	}
}

// Build:   CGO_ENABLED=1 go build -o waddler ./cmd/waddler
// Install: CGO_ENABLED=1 go install ./cmd/waddler
