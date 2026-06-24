package main

import (
	"os"
	"testing"
)

func TestMainHelp(t *testing.T) {
	old := os.Args
	os.Args = []string{"dreamland", "--help"}
	defer func() { os.Args = old }()
	main() // --help returns nil; no os.Exit called
}
