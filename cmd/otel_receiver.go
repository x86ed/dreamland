package cmd

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"dreamland/internal/config"
	"dreamland/internal/telemetry/otelreceiver"
)

var otelReceiverCmd = &cobra.Command{
	Use:   "otel-receiver",
	Short: "Start a local OTLP/HTTP trace receiver for GitHub Copilot token usage (no-op if already running)",
	RunE:  runOtelReceiver,
}

var otelReceiverForeground bool

func init() {
	rootCmd.AddCommand(otelReceiverCmd)
	otelReceiverCmd.Flags().BoolVar(&otelReceiverForeground, "foreground", false,
		"run the receiver loop in this process instead of spawning a detached background process (internal use)")
}

func runOtelReceiver(_ *cobra.Command, _ []string) error {
	cwd, err := osGetwd()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	repoRoot, err := config.FindRepoRoot(cwd)
	if err != nil {
		return err
	}

	addr := otelReceiverAddr(cfg.OtelEndpoint)

	if otelReceiverForeground {
		srv := &http.Server{Addr: addr, Handler: otelreceiver.Handler(repoRoot)}
		return srv.ListenAndServe()
	}

	// Idempotent: if something is already listening on this address (almost certainly a
	// prior instance of this same receiver, started at an earlier SessionStart), do nothing.
	if conn, dialErr := net.DialTimeout("tcp", addr, 200*time.Millisecond); dialErr == nil {
		conn.Close()
		return nil
	}

	// Spawn a detached child running the real server loop, then return immediately —
	// this command is invoked from a SessionStart hook, which must not block the session
	// waiting for a long-running server.
	exe, err := os.Executable()
	if err != nil {
		exe = "dreamland"
	}
	child := exec.Command(exe, "otel-receiver", "--foreground")
	child.Dir = repoRoot
	detachProcess(child)
	if startErr := child.Start(); startErr != nil {
		return nil // best-effort — a missing receiver must never fail the hook chain
	}
	_ = child.Process.Release()
	return nil
}

// otelReceiverAddr derives the listen address (host:port) from the configured OTLP
// endpoint, applying the same gRPC(4317)->HTTP(4318) port translation used when writing
// github.copilot.chat.otel.otlpEndpoint (see copilotOtelEndpoint in internal/scaffold).
func otelReceiverAddr(otelEndpoint string) string {
	endpoint := otelEndpoint
	if endpoint == "" {
		endpoint = "http://localhost:4317"
	}
	endpoint = strings.ReplaceAll(endpoint, ":4317", ":4318")

	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return "localhost:4318"
	}
	return u.Host
}
