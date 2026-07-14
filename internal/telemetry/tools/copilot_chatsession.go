package tools

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// requestUsage accumulates the token counts VS Code's chat-session log records for one
// request (one turn) within a session.
type requestUsage struct {
	PromptTokens     int64
	CompletionTokens int64
}

// findChatSessionFile locates VS Code's own persisted chat-session log for sessionID.
// This is the real, verified source of GitHub Copilot token usage: neither the
// SubagentStop/Stop hook payload, the transcript file, nor Copilot's OTel export
// (confirmed empirically: nothing arrives at a real running OTLP receiver) carry usage
// data, but VS Code's own workspaceStorage/<hash>/chatSessions/<sessionID>.jsonl does —
// confirmed by grepping a real file for "promptTokens"/"completionTokens" and finding
// real non-zero values. Session IDs are UUIDs (globally unique), so searching across
// every workspace's storage directory for a matching filename is reliable without needing
// to reverse-engineer VS Code's workspace-path-to-storage-hash algorithm.
func findChatSessionFile(sessionID string) (string, error) {
	if sessionID == "" {
		return "", os.ErrNotExist
	}
	for _, root := range vscodeWorkspaceStorageRoots() {
		matches, err := filepath.Glob(filepath.Join(root, "*", "chatSessions", sessionID+".jsonl"))
		if err != nil {
			continue
		}
		if len(matches) > 0 {
			return matches[0], nil
		}
	}
	return "", os.ErrNotExist
}

func vscodeWorkspaceStorageRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	switch runtime.GOOS {
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Application Support", "Code", "User", "workspaceStorage"),
			filepath.Join(home, "Library", "Application Support", "Code - Insiders", "User", "workspaceStorage"),
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return nil
		}
		return []string{
			filepath.Join(appData, "Code", "User", "workspaceStorage"),
			filepath.Join(appData, "Code - Insiders", "User", "workspaceStorage"),
		}
	default: // linux and other unix-likes
		return []string{
			filepath.Join(home, ".config", "Code", "User", "workspaceStorage"),
			filepath.Join(home, ".config", "Code - Insiders", "User", "workspaceStorage"),
		}
	}
}

// parseChatSessionTokens replays VS Code's incremental JSON-patch chat-session log and
// returns the prompt/completion token counts for the most recently completed request
// (the highest request index that has usage data) — i.e. the turn that just ended, which
// is what triggered the SubagentStop/Stop hook invoking this in the first place.
//
// Each line is a patch: {"kind": <1|2>, "k": [<path segments>], "v": <value>}. Usage shows
// up two ways, both handled: a direct patch to "requests.<n>.promptTokens" /
// "requests.<n>.completionTokens", and a nested patch to "requests.<n>.result" whose value
// contains "metadata.promptTokens" / "metadata.outputTokens" (same data, different key
// name for the output-token count). Unknown "kind" values and unrelated paths are ignored.
func parseChatSessionTokens(path string) (promptTokens, completionTokens int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	requests := map[int]*requestUsage{}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var patch struct {
			K []any           `json:"k"`
			V json.RawMessage `json:"v"`
		}
		if jsonErr := json.Unmarshal(scanner.Bytes(), &patch); jsonErr != nil {
			continue
		}
		if len(patch.K) < 3 {
			continue
		}
		if s, ok := patch.K[0].(string); !ok || s != "requests" {
			continue
		}
		idxF, ok := patch.K[1].(float64)
		if !ok {
			continue
		}
		idx := int(idxF)
		field, ok := patch.K[2].(string)
		if !ok {
			continue
		}

		if requests[idx] == nil {
			requests[idx] = &requestUsage{}
		}

		switch field {
		case "promptTokens":
			if n, ok := decodeJSONInt(patch.V); ok {
				requests[idx].PromptTokens = n
			}
		case "completionTokens":
			if n, ok := decodeJSONInt(patch.V); ok {
				requests[idx].CompletionTokens = n
			}
		case "result":
			var res struct {
				Metadata struct {
					PromptTokens int64 `json:"promptTokens"`
					OutputTokens int64 `json:"outputTokens"`
				} `json:"metadata"`
			}
			if jsonErr := json.Unmarshal(patch.V, &res); jsonErr == nil {
				if res.Metadata.PromptTokens > 0 {
					requests[idx].PromptTokens = res.Metadata.PromptTokens
				}
				if res.Metadata.OutputTokens > 0 {
					requests[idx].CompletionTokens = res.Metadata.OutputTokens
				}
			}
		}
	}

	bestIdx := -1
	var best *requestUsage
	for idx, ru := range requests {
		if (ru.PromptTokens > 0 || ru.CompletionTokens > 0) && idx > bestIdx {
			best, bestIdx = ru, idx
		}
	}
	if best == nil {
		return 0, 0, nil
	}
	return best.PromptTokens, best.CompletionTokens, nil
}

func decodeJSONInt(raw json.RawMessage) (int64, bool) {
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, false
	}
	return int64(f), true
}
