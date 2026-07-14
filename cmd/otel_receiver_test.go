package cmd

import (
	"net"
	"testing"
	"time"

	"dreamland/internal/config"
)

func TestOtelReceiverAddr_Default(t *testing.T) {
	if got := otelReceiverAddr(""); got != "localhost:4318" {
		t.Errorf("got %q, want localhost:4318", got)
	}
}

func TestOtelReceiverAddr_TranslatesGRPCPort(t *testing.T) {
	if got := otelReceiverAddr("http://localhost:4317"); got != "localhost:4318" {
		t.Errorf("got %q, want localhost:4318", got)
	}
}

func TestOtelReceiverAddr_CustomPortPreserved(t *testing.T) {
	if got := otelReceiverAddr("http://localhost:9999"); got != "localhost:9999" {
		t.Errorf("got %q, want localhost:9999", got)
	}
}

func TestOtelReceiverAddr_InvalidURLFallsBack(t *testing.T) {
	if got := otelReceiverAddr("://not a url"); got != "localhost:4318" {
		t.Errorf("got %q, want localhost:4318 fallback", got)
	}
}

func TestRunOtelReceiver_NoOpWhenAlreadyListening(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	root := makeCoauthorRepo(t, config.Config{
		CodingTool:   "GitHub Copilot",
		OtelEndpoint: "http://" + ln.Addr().String(),
	})
	_ = root

	origForeground := otelReceiverForeground
	otelReceiverForeground = false
	t.Cleanup(func() { otelReceiverForeground = origForeground })

	done := make(chan error, 1)
	go func() { done <- runOtelReceiver(nil, nil) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runOtelReceiver did not return promptly when a receiver was already listening")
	}
}
