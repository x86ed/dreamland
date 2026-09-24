package handoff

import (
	"regexp"
	"strings"
)

// Tags are the machine-readable own-line tags a subagent ends its report with.
// A value outside the closed set for its key is left empty (treated as absent).
type Tags struct {
	Handoff string
	Verdict string
	Change  string
}

var tagLine = regexp.MustCompile(`^\s*\[(handoff|verdict|change): ([a-z0-9-]+)\]\s*$`)

var (
	handoffValues = map[string]bool{"complete": true, "blocked": true}
	verdictValues = map[string]bool{"pass": true, "fail": true, "spec-defect": true, "unverified": true}
)

// ParseTags scans report line by line; the last matching line per key wins, so a
// tag quoted inline in prose, or earlier in the report, cannot steer the result.
func ParseTags(report string) Tags {
	var t Tags
	report = strings.ReplaceAll(report, "\r\n", "\n")
	for _, line := range strings.Split(report, "\n") {
		m := tagLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		switch m[1] {
		case "handoff":
			if handoffValues[m[2]] {
				t.Handoff = m[2]
			} else {
				t.Handoff = ""
			}
		case "verdict":
			if verdictValues[m[2]] {
				t.Verdict = m[2]
			} else {
				t.Verdict = ""
			}
		case "change":
			if ValidChange(m[2]) {
				t.Change = m[2]
			} else {
				t.Change = ""
			}
		}
	}
	return t
}
