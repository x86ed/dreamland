package handoff

const (
	KindDispatch = "dispatch"
	KindReport   = "report"
)

// Directive is what the dispatcher must do next. A zero Directive (empty Kind)
// means no directive.
type Directive struct {
	Kind       string `json:"kind,omitempty"`
	Target     string `json:"target,omitempty"`
	Reason     string `json:"reason,omitempty"`
	TagMissing bool   `json:"tag_missing,omitempty"`
	// ClearCounter is set when the change's failure counter must be deleted.
	ClearCounter bool `json:"-"`
}

// Counter is the per-change state persisted between subagent turns.
type Counter struct {
	PhobetorFailures int `json:"phobetor_failures"`
	VerdictRetries   int `json:"verdict_retries"`
}

// completeEdges maps an agent to the agent that must run next when it completes.
// The nyx row is a drafted, unconfirmed decision; deleting it removes the edge.
var completeEdges = map[string]string{
	"nyx":      "morpheus",
	"morpheus": "phobetor",
	"iktomi":   "phobetor",
}

// Targets returns every agent the table can direct a dispatcher to, per source
// agent, for the drift test.
func Targets() map[string][]string {
	return map[string][]string{
		"nyx":      {"morpheus"},
		"morpheus": {"phobetor"},
		"iktomi":   {"phobetor"},
		"phobetor": {"baku", "morpheus", "phantasos"},
	}
}

// Next is the single decision function over the edge table. It returns the
// directive and the counter after applying the outcome; it does no I/O.
func Next(from string, tags Tags, c Counter) (Directive, Counter) {
	if target, ok := completeEdges[from]; ok {
		if tags.Handoff == "blocked" {
			return Directive{Kind: KindReport, Reason: from + " reported blocked; surface the blocker to Janus"}, c
		}
		return Directive{
			Kind:       KindDispatch,
			Target:     target,
			Reason:     from + " completed",
			TagMissing: tags.Handoff == "",
		}, c
	}
	switch from {
	case "phobetor":
		return nextPhobetor(tags, c)
	case "phantasos":
		return Directive{ClearCounter: true}, Counter{}
	case "baku":
		return Directive{ClearCounter: true}, Counter{}
	}
	return Directive{}, c
}

func nextPhobetor(tags Tags, c Counter) (Directive, Counter) {
	switch tags.Verdict {
	case "pass":
		return Directive{Kind: KindDispatch, Target: "baku", Reason: "phobetor verdict pass", ClearCounter: true}, Counter{}
	case "fail":
		c.VerdictRetries = 0
		if c.PhobetorFailures == 0 {
			c.PhobetorFailures = 1
			return Directive{Kind: KindDispatch, Target: "morpheus", Reason: "phobetor verdict fail (first failure)"}, c
		}
		c.PhobetorFailures = 2
		return Directive{Kind: KindDispatch, Target: "phantasos", Reason: "phobetor verdict fail after a morpheus retry"}, c
	case "spec-defect":
		c.VerdictRetries = 0
		return Directive{Kind: KindDispatch, Target: "phantasos", Reason: "phobetor verdict spec-defect"}, c
	case "unverified":
		c.VerdictRetries = 0
		return Directive{Kind: KindReport, Reason: "phobetor could not run the tests (unverified); surface this to Janus"}, c
	}
	if c.VerdictRetries == 0 {
		c.VerdictRetries = 1
		return Directive{
			Kind:       KindDispatch,
			Target:     "phobetor",
			Reason:     "phobetor report had no valid verdict tag; run again and end your report with exactly one [verdict: pass|fail|spec-defect|unverified] line",
			TagMissing: true,
		}, c
	}
	return Directive{Kind: KindReport, Reason: "phobetor again produced no valid verdict tag; surface this to Janus", TagMissing: true}, c
}

// KnownAgent reports whether from appears in the edge table.
func KnownAgent(from string) bool {
	if _, ok := completeEdges[from]; ok {
		return true
	}
	return TouchesCounter(from)
}

// TouchesCounter reports whether from's outcome reads or writes the failure counter.
func TouchesCounter(from string) bool {
	return from == "phobetor" || from == "phantasos" || from == "baku"
}
