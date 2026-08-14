package cmd

import (
	"encoding/json"
	"io"
)

// readHookPayload reads a JSON hook payload from r and unmarshals it into a generic
// map. Returns nil if the read or unmarshal fails, or the payload is empty — every
// caller treats that as "no payload arrived", never as an error to surface, since hook
// payloads are inherently best-effort input.
func readHookPayload(r io.Reader) map[string]any {
	data, err := io.ReadAll(io.LimitReader(r, 1<<16))
	if err != nil || len(data) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	return payload
}
