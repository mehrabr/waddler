// Package hub serves a file-backed DuckDB database over DuckDB's native Quack
// protocol, so other waddler runs can push results to a shared "hub" with a
// `quack` output. It replaces the old hand-rolled HTTP relay: the transport,
// auth, and wire protocol are all DuckDB's, not ours.
package hub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mehrabr/waddler/internal/engine"
	"github.com/mehrabr/waddler/internal/sqlutil"
)

type Config struct {
	DBPath    string // persistent DuckDB file the hub serves
	Listen    string // host:port to bind (e.g. 0.0.0.0:9494)
	TokenFile string // file holding the auth token (created on first start)
}

// Serve starts the Quack server and blocks until interrupted. The server runs
// in the background inside the DuckDB process, so after quack_serve returns we
// simply wait for a signal.
func Serve(cfg Config) error {
	eng, err := engine.NewWithFile(cfg.DBPath)
	if err != nil {
		return err
	}
	defer eng.Close()

	if err := eng.RequireDuckDB(1, 5, 3); err != nil {
		return fmt.Errorf("hub: %w", err)
	}
	for _, stmt := range []string{"INSTALL quack", "LOAD quack"} {
		if err := eng.Exec(stmt); err != nil {
			return fmt.Errorf("hub: load quack extension: %w", err)
		}
	}

	token, err := loadOrCreateToken(cfg.TokenFile)
	if err != nil {
		return err
	}

	addr := cfg.Listen
	if !strings.HasPrefix(addr, "quack:") {
		addr = "quack:" + addr
	}
	// allow_other_hostname lets clients reach the hub by something other than
	// localhost; token pins a stable auth token across restarts.
	call := fmt.Sprintf(
		"CALL quack_serve(%s, allow_other_hostname => true, token := %s)",
		sqlutil.QuoteLiteral(addr), sqlutil.QuoteLiteral(token),
	)
	var listenURI, url, authToken string
	if err := eng.QueryRow(call, &listenURI, &url, &authToken); err != nil {
		return fmt.Errorf("hub: start quack server: %w", err)
	}

	slog.Info("quack hub serving", "db", cfg.DBPath, "listen", listenURI, "token_file", cfg.TokenFile)
	fmt.Printf("🦆 Quack hub serving %s\n", cfg.DBPath)
	fmt.Printf("   listen: %s\n", listenURI)
	fmt.Printf("   token : stored in %s (share it with clients as WADDLER_QUACK_TOKEN)\n", cfg.TokenFile)
	fmt.Printf("   Press Ctrl+C to stop.\n")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	fmt.Println("\nshutting down hub")
	return nil
}

// loadOrCreateToken returns the token in path, generating and persisting a new
// one (0600) if the file is missing or empty.
func loadOrCreateToken(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if tok := strings.TrimSpace(string(b)); tok != "" {
			return tok, nil
		}
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("hub: generate token: %w", err)
	}
	tok := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(tok+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("hub: write token file %q: %w", path, err)
	}
	slog.Info("generated new hub token", "file", path)
	return tok, nil
}
