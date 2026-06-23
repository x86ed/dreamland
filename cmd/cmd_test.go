package cmd

import (
	"os"
	"testing"
	"time"
)

// TestExecute verifies Execute runs without panicking when --help is passed.
func TestExecute(t *testing.T) {
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"dreamland", "--help"}
	Execute()
}

// TestRunServeExitsOnStdinEOF verifies runServe exits when stdin is closed,
// which is the normal signal that an MCP client has disconnected.
func TestRunServeExitsOnStdinEOF(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.Close() // immediate EOF

	oldStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		r.Close()
	}()

	done := make(chan error, 1)
	go func() { done <- runServe(serveCmd, nil) }()

	select {
	case <-done:
		// returned as expected on EOF
	case <-time.After(10 * time.Second):
		t.Error("runServe did not exit within 10s after stdin EOF")
	}
}
