package cmd

import (
	"reflect"
	"testing"
)

func TestSplitWindowsCommandLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"quoted program with spaces", `"C:\Program Files\dreamland\dreamland.exe" otel-receiver --foreground --addr localhost:4318`,
			[]string{"otel-receiver", "--foreground", "--addr", "localhost:4318"}},
		{"unquoted program", `C:\x\dreamland.exe otel-receiver`, []string{"otel-receiver"}},
		{"quoted argument", `dreamland.exe "otel receiver"`, []string{"otel receiver"}},
		{"program only", `dreamland.exe`, nil},
		{"empty", ``, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitWindowsCommandLine(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitWindowsCommandLine(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParsePSCommandLine_LinuxShortCommWithSpacedPath(t *testing.T) {
	exe, args, err := parsePSCommandLine("dreamland", "/Users/x/My Tools/dreamland otel-receiver --foreground")
	if err != nil {
		t.Fatal(err)
	}
	if exe != "dreamland" || !reflect.DeepEqual(args, []string{"otel-receiver", "--foreground"}) {
		t.Errorf("exe=%q args=%q, want dreamland / [otel-receiver --foreground]", exe, args)
	}
}
