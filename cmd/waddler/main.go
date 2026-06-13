package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/mehrabr/waddler/internal/config"
	"github.com/mehrabr/waddler/internal/hub"
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
	root.AddCommand(cmdRun(), cmdValidate(), cmdServe(), cmdSources(), cmdHub())
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
			fmt.Printf("\n✅ %s\n", result)
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
				return fmt.Errorf("no 'schedule' field in pipeline — add e.g. schedule: \"0 6 * * *\"")
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
			fmt.Printf("⏰ Scheduler running — %s on %q. Press Ctrl+C to stop.\n", p.Name, p.Schedule)
			select {}
		},
	}
}

func cmdHub() *cobra.Command {
	var (
		dbPath    string
		listen    string
		tokenFile string
	)
	cmd := &cobra.Command{
		Use:   "hub",
		Short: "Serve a DuckDB database over Quack so other waddler runs can push to it",
		Long: "Serve a persistent DuckDB file over DuckDB's native Quack protocol.\n" +
			"Branch pipelines push results to it with a `quack` output. The auth\n" +
			"token is written to --token-file on first start; share it with clients\n" +
			"as the WADDLER_QUACK_TOKEN environment variable.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return hub.Serve(hub.Config{DBPath: dbPath, Listen: listen, TokenFile: tokenFile})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "./hub.duckdb", "path to the persistent DuckDB file the hub serves")
	cmd.Flags().StringVar(&listen, "listen", "0.0.0.0:9494", "host:port to serve Quack on")
	cmd.Flags().StringVar(&tokenFile, "token-file", "./hub.token", "file holding the hub auth token (created on first start)")
	return cmd
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
  postgres   — PostgreSQL table (dsn + table required; dsn may use ${VAR})
  motherduck — MotherDuck cloud table (table required; MOTHERDUCK_TOKEN env or token: ${VAR})
  quack      — table on a waddler hub over Quack (url + table required; WADDLER_QUACK_TOKEN env or token: ${VAR})

Outputs:
  parquet    — local Parquet file (path required; optional compression)
  csv        — local CSV file (path required)
  motherduck — MotherDuck cloud table (table required; mode: replace|append)
  quack      — table on a waddler hub over Quack (url + table required; mode: replace|append)

Schedule field (optional):
  Standard cron expression, e.g. "0 6 * * *" (daily at 6am).
  Use with: waddler serve pipeline.yml

Run a hub:
  waddler hub --listen 0.0.0.0:9494 --token-file hub.token`)
		},
	}
}

// Build:   CGO_ENABLED=1 go build -o waddler ./cmd/waddler
// Install: CGO_ENABLED=1 go install ./cmd/waddler
