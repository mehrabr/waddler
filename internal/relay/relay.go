package relay

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mehrabr/waddler/internal/engine"
)

// Config holds the settings for a relay server.
type Config struct {
	DBPath           string
	Port             int
	TokenFile        string
	AllowedPipelines []string // empty means all pipelines are allowed
	MaxRows          int64
}

// LoadOrCreateToken reads the token from TokenFile, creating a random one
// and writing it to the file if it doesn't exist yet.
func LoadOrCreateToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return strings.TrimSpace(string(data)), nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("relay: read token file: %w", err)
	}
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("relay: write token file: %w", err)
	}
	return token, nil
}

// AllowedPipeline reports whether name is on the allowlist.
// If the allowlist is empty, all pipelines are allowed.
func (c *Config) AllowedPipeline(name string) bool {
	if len(c.AllowedPipelines) == 0 {
		return true
	}
	for _, a := range c.AllowedPipelines {
		if a == name {
			return true
		}
	}
	return false
}

// pipelineRunRequest is the JSON body sent by waddler run when writing to a relay.
type pipelineRunRequest struct {
	Pipeline  string `json:"pipeline"`
	Transform string `json:"transform"`
	Table     string `json:"table"`
	Database  string `json:"database"`
	Mode      string `json:"mode"`
	RowCount  int64  `json:"row_count"`
}

// Serve starts the relay HTTP server. It blocks until the process is killed.
func Serve(cfg Config) error {
	eng, err := engine.NewWithFile(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("relay: open database: %w", err)
	}
	defer eng.Close()

	token, err := LoadOrCreateToken(cfg.TokenFile)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/write", func(w http.ResponseWriter, r *http.Request) {
		handleWrite(w, r, eng, cfg, token)
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("relay listening", "addr", addr)
	return http.ListenAndServe(addr, mux)
}

// NewTestServer returns an http.Handler wired to eng — useful in tests to avoid
// opening a real TCP listener.
func NewTestServer(eng *engine.Engine, cfg Config, token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/write", func(w http.ResponseWriter, r *http.Request) {
		handleWrite(w, r, eng, cfg, token)
	})
	return mux
}

func handleWrite(w http.ResponseWriter, r *http.Request, eng *engine.Engine, cfg Config, token string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("Authorization") != "Bearer "+token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req pipelineRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if !cfg.AllowedPipeline(req.Pipeline) {
		slog.Warn("relay: rejected pipeline not on allowlist", "pipeline", req.Pipeline)
		http.Error(w, fmt.Sprintf("pipeline %q is not on the allowed list", req.Pipeline), http.StatusForbidden)
		return
	}

	if req.RowCount > cfg.MaxRows {
		slog.Warn("relay: rejected run exceeding row limit", "pipeline", req.Pipeline, "rows", req.RowCount, "limit", cfg.MaxRows)
		http.Error(w, fmt.Sprintf("row count %d exceeds relay limit %d", req.RowCount, cfg.MaxRows), http.StatusRequestEntityTooLarge)
		return
	}

	start := time.Now()

	dbName := req.Database
	if dbName == "" {
		dbName = "main"
	}
	table := fmt.Sprintf("%s.%s", dbName, req.Table)

	var stmt string
	if req.Mode == "append" {
		stmt = fmt.Sprintf("INSERT INTO %s BY NAME (%s)", table, req.Transform)
	} else {
		stmt = fmt.Sprintf("CREATE OR REPLACE TABLE %s AS (%s)", table, req.Transform)
	}

	if err := eng.Exec(stmt); err != nil {
		slog.Error("relay: pipeline write failed", "pipeline", req.Pipeline, "err", err)
		http.Error(w, "write failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("relay: pipeline written",
		"pipeline", req.Pipeline,
		"table", table,
		"rows", req.RowCount,
		"duration", time.Since(start).Round(time.Millisecond),
	)
	w.WriteHeader(http.StatusOK)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("relay: generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
