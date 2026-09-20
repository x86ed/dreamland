package cmd

import (
	"reflect"
	"testing"
)

// These parsers are pure Go and run on every platform, so the Windows netstat format is
// verified on Unix CI as well.

func TestParseLsofPIDs(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    []int
		wantErr bool
	}{
		{"single pid", "1234\n", []int{1234}, false},
		{"no trailing newline", "1234", []int{1234}, false},
		{"IPv4 and IPv6 list the same pid twice", "1234\n1234\n", []int{1234}, false},
		{"two distinct pids", "1234\n5678\n", []int{1234, 5678}, false},
		{"distinct pids come back ascending and deduplicated", "5678\n1234\n5678\n", []int{1234, 5678}, false},
		{"CRLF and blank lines", "\r\n1234\r\n\r\n", []int{1234}, false},
		{"empty output means no listener", "", nil, false},
		{"whitespace only", " \n\n", nil, false},
		{"unparseable text", "lsof: WARNING: can't stat() fuse file system\n", nil, true},
		{"mixed pid and text", "1234\nnot-a-pid\n", nil, true},
		{"negative pid", "-5\n", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLsofPIDs(tt.out)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) && !(len(got) == 0 && len(tt.want) == 0) {
				t.Errorf("pids = %v, want %v", got, tt.want)
			}
		})
	}
}

const netstatHeader = `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
`

func TestParseNetstatPIDs(t *testing.T) {
	tests := []struct {
		name string
		out  string
		port int
		want []int
	}{
		{
			name: "IPv4 listener",
			out:  netstatHeader + "  TCP    127.0.0.1:4318         0.0.0.0:0              LISTENING       1234\r\n",
			port: 4318, want: []int{1234},
		},
		{
			name: "IPv4 and IPv6 listeners of one process are deduplicated",
			out: netstatHeader +
				"  TCP    0.0.0.0:4318           0.0.0.0:0              LISTENING       1234\r\n" +
				"  TCP    [::]:4318              [::]:0                 LISTENING       1234\r\n" +
				"  TCP    [::1]:4318             [::]:0                 LISTENING       1234\r\n",
			port: 4318, want: []int{1234},
		},
		{
			name: "two distinct pids are both reported",
			out: netstatHeader +
				"  TCP    127.0.0.1:4318         0.0.0.0:0              LISTENING       1234\r\n" +
				"  TCP    [::1]:4318             [::]:0                 LISTENING       5678\r\n",
			port: 4318, want: []int{1234, 5678},
		},
		{
			name: "similar ports do not match",
			out: netstatHeader +
				"  TCP    127.0.0.1:43180        0.0.0.0:0              LISTENING       1111\r\n" +
				"  TCP    127.0.0.1:14318        0.0.0.0:0              LISTENING       2222\r\n" +
				"  TCP    127.0.0.1:431          0.0.0.0:0              LISTENING       3333\r\n",
			port: 4318, want: nil,
		},
		{
			name: "established connections to the port are not listeners",
			out: netstatHeader +
				"  TCP    127.0.0.1:52000        127.0.0.1:4318         ESTABLISHED     4444\r\n" +
				"  TCP    127.0.0.1:4318         127.0.0.1:52000        ESTABLISHED     1234\r\n" +
				"  TCP    127.0.0.1:4318         0.0.0.0:0              LISTENING       1234\r\n",
			port: 4318, want: []int{1234},
		},
		{
			name: "UDP endpoints are ignored",
			out:  netstatHeader + "  UDP    0.0.0.0:4318           *:*                                    9999\r\n",
			port: 4318, want: nil,
		},
		{name: "empty output", out: "", port: 4318, want: nil},
		{name: "headers only", out: netstatHeader, port: 4318, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNetstatPIDs(tt.out, tt.port)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("pids = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePSCommandLine(t *testing.T) {
	tests := []struct {
		name     string
		comm     string
		argsLine string
		wantExe  string
		wantArgs []string
		wantErr  bool
	}{
		{
			name: "macOS: comm is the full path", comm: "/usr/local/bin/dreamland\n",
			argsLine: "/usr/local/bin/dreamland otel-receiver --foreground --addr localhost:4318\n",
			wantExe:  "/usr/local/bin/dreamland", wantArgs: []string{"otel-receiver", "--foreground", "--addr", "localhost:4318"},
		},
		{
			name: "executable path containing spaces", comm: "/Users/x/My Tools/dreamland",
			argsLine: "/Users/x/My Tools/dreamland otel-receiver --foreground",
			wantExe:  "/Users/x/My Tools/dreamland", wantArgs: []string{"otel-receiver", "--foreground"},
		},
		{
			name: "Linux: comm is only the short name", comm: "dreamland",
			argsLine: "/usr/local/bin/dreamland otel-receiver",
			wantExe:  "dreamland", wantArgs: []string{"otel-receiver"},
		},
		{
			name: "no arguments", comm: "/usr/local/bin/dreamland",
			argsLine: "/usr/local/bin/dreamland",
			wantExe:  "/usr/local/bin/dreamland", wantArgs: nil,
		},
		{name: "empty comm is an error", comm: "", argsLine: "x otel-receiver", wantErr: true},
		{name: "blank comm is an error", comm: "  \n", argsLine: "x otel-receiver", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exe, args, err := parsePSCommandLine(tt.comm, tt.argsLine)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if exe != tt.wantExe {
				t.Errorf("exe = %q, want %q", exe, tt.wantExe)
			}
			if len(args) == 0 && len(tt.wantArgs) == 0 {
				return
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("args = %q, want %q", args, tt.wantArgs)
			}
		})
	}
}
