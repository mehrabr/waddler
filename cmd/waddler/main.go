package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/mehrabr/waddler/internal/config"
	"github.com/mehrabr/waddler/internal/relay"
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
	root.AddCommand(cmdRun(), cmdValidate(), cmdServe(), cmdSources(), cmdRelay())
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

func cmdServe() *cobra.Command {
	return &cobra.Command{
		Use:   "serve [pipeline.yml]",
		Short: "Run a pipeline on its configured schedule (blocks until Ctrl+C)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.Load(args[0])
			if err != nil {
				return err
			}
			if !p.HasSchedule() {
				return fmt.Errorf(
					"no 'schedule' field in pipeline — add e.g. schedule: \"0 6 * * *\"",
				)
			}
			c := cron.New()
			if _, err := c.AddFunc(p.Schedule, func() {
				if _, err := runner.Run(p); err != nil {
					slog.Error("pipeline failed", "err", err)
				}
			}); err != nil {
				return fmt.Errorf("invalid cron expression %q: %w", p.Schedule, err)
			}
			c.Start()
			slog.Info("scheduler started", "schedule", p.Schedule, "pipeline", p.Name)
			fmt.Printf("⏰ Scheduler running — %s on %q. Press Ctrl+C to stop.\n",
				p.Name, p.Schedule)
			select {}
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
  quack      — remote waddler relay or Quack server (url + table required)

Outputs:
  parquet    — local Parquet file (path required)
  csv        — local CSV file (path required)
  motherduck — MotherDuck cloud table (database + table required)
  quack      — remote waddler relay or Quack server (url + table required)

Schedule field (optional):
  Standard cron expression, e.g. "0 6 * * *" (daily at 6am).
  Use with: waddler serve pipeline.yml`)
		},
	}
}

func cmdRelay() *cobra.Command {
	var (
		dbPath           string
		port             int
		tokenFile        string
		allowedPipelines string
		maxRows          int64
	)

	cmd := &cobra.Command{
		Use:   "relay",
		Short: "Start a Quack-backed hub that accepts pipeline writes from remote waddler instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := relay.Config{
				DBPath:    dbPath,
				Port:      port,
				TokenFile: tokenFile,
				MaxRows:   maxRows,
			}
			if allowedPipelines != "" {
				for _, name := range strings.Split(allowedPipelines, ",") {
					cfg.AllowedPipelines = append(cfg.AllowedPipelines, strings.TrimSpace(name))
				}
			}

			if len(cfg.AllowedPipelines) == 0 {
				slog.Warn("WARNING: no --allowed-pipelines set. Any client with the token can write any pipeline to this relay. Set --allowed-pipelines in production.")
			}

			slog.Info("relay starting",
				"db", dbPath,
				"port", port,
				"token_file", tokenFile,
				"max_rows", maxRows,
			)
			return relay.Serve(cfg)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "./hub.duckdb", "path to the persistent DuckDB file")
	cmd.Flags().IntVar(&port, "port", 9494, "port to listen on")
	cmd.Flags().StringVar(&tokenFile, "token-file", "./relay.token", "file containing the relay auth token (created on first start)")
	cmd.Flags().StringVar(&allowedPipelines, "allowed-pipelines", "", "comma-separated list of pipeline names allowed to write to this relay")
	cmd.Flags().Int64Var(&maxRows, "max-rows", 10_000_000, "maximum rows accepted per pipeline run")

	return cmd
}

// Build:   CGO_ENABLED=1 go build -o waddler ./cmd/waddler
// Install: CGO_ENABLED=1 go install ./cmd/waddler
