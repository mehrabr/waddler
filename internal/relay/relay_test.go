package relay_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mehrabr/waddler/internal/engine"
	"github.com/mehrabr/waddler/internal/relay"
)

func TestLoadOrCreateToken_CreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.token")
	token, err := relay.LoadOrCreateToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) == 0 {
		t.Error("expected non-empty token")
	}
	// Second call should return the same token.
	token2, err := relay.LoadOrCreateToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if token != token2 {
		t.Errorf("token changed between reads: %q vs %q", token, token2)
	}
}

func TestLoadOrCreateToken_FilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.token")
	if _, err := relay.LoadOrCreateToken(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected 0600 permissions, got %o", info.Mode().Perm())
	}
}

func TestConfig_AllowedPipeline(t *testing.T) {
	cfg := relay.Config{AllowedPipelines: []string{"donor_report", "weekly_sync"}}

	if !cfg.AllowedPipeline("donor_report") {
		t.Error("expected donor_report to be allowed")
	}
	if cfg.AllowedPipeline("other_pipeline") {
		t.Error("expected other_pipeline to be rejected")
	}
}

func TestConfig_AllowedPipeline_EmptyListAllowsAll(t *testing.T) {
	cfg := relay.Config{}
	if !cfg.AllowedPipeline("anything") {
		t.Error("expected all pipelines allowed when list is empty")
	}
}

func TestHandleWrite_RejectsWrongMethod(t *testing.T) {
	eng := mustNewEngine(t)
	cfg := relay.Config{MaxRows: 1_000_000}
	srv := relay.NewTestServer(eng, cfg, "testtoken")

	req := httptest.NewRequest(http.MethodGet, "/write", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleWrite_RejectsBadToken(t *testing.T) {
	eng := mustNewEngine(t)
	cfg := relay.Config{MaxRows: 1_000_000}
	srv := relay.NewTestServer(eng, cfg, "correcttoken")

	body := mustMarshal(t, map[string]any{
		"pipeline": "test", "transform": "SELECT 1", "table": "t", "row_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/write", body)
	req.Header.Set("Authorization", "Bearer wrongtoken")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleWrite_RejectsDisallowedPipeline(t *testing.T) {
	eng := mustNewEngine(t)
	cfg := relay.Config{
		AllowedPipelines: []string{"allowed_only"},
		MaxRows:          1_000_000,
	}
	srv := relay.NewTestServer(eng, cfg, "tok")

	body := mustMarshal(t, map[string]any{
		"pipeline": "not_allowed", "transform": "SELECT 1", "table": "t", "row_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/write", body)
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestHandleWrite_RejectsExcessiveRowCount(t *testing.T) {
	eng := mustNewEngine(t)
	cfg := relay.Config{MaxRows: 100}
	srv := relay.NewTestServer(eng, cfg, "tok")

	body := mustMarshal(t, map[string]any{
		"pipeline": "test", "transform": "SELECT 1", "table": "t", "row_count": 1000,
	})
	req := httptest.NewRequest(http.MethodPost, "/write", body)
	req.Header.Set("Authorization", "Bearer tok")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", w.Code)
	}
}

func TestHandleWrite_AcceptsValidRequest(t *testing.T) {
	eng := mustNewEngine(t)
	cfg := relay.Config{MaxRows: 1_000_000}
	srv := relay.NewTestServer(eng, cfg, "tok")

	body := mustMarshal(t, map[string]any{
		"pipeline":  "test",
		"transform": "SELECT 42 AS val",
		"table":     "results",
		"row_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/write", body)
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func mustNewEngine(t *testing.T) *engine.Engine {
	t.Helper()
	eng, err := engine.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Close() })
	return eng
}

func mustMarshal(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewBuffer(b)
}
