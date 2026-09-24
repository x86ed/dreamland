package zhougongdata

import (
	"sort"
	"strings"
)

// Transition counts agent-to-agent handoffs between consecutive runs.
type Transition struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int    `json:"count"`
}

// FlowPath returns the ordered agent sequence of runs, collapsing consecutive duplicates.
func FlowPath(runs []Run) []string {
	path := []string{}
	for _, r := range runs {
		if len(path) == 0 || path[len(path)-1] != r.Agent {
			path = append(path, r.Agent)
		}
	}
	return path
}

// Transitions counts each consecutive agent pair in the flow path, sorted by count
// descending then by first occurrence.
func Transitions(runs []Run) []Transition {
	path := FlowPath(runs)
	idx := map[string]int{}
	var out []Transition
	for i := 0; i+1 < len(path); i++ {
		k := path[i] + "\x00" + path[i+1]
		if j, ok := idx[k]; ok {
			out[j].Count++
			continue
		}
		idx[k] = len(out)
		out = append(out, Transition{From: path[i], To: path[i+1], Count: 1})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if out == nil {
		out = []Transition{}
	}
	return out
}

// TypicalFlow returns the most frequent flow path across datasets, ties broken by
// earliest occurrence.
func TypicalFlow(datasets []Dataset) []string {
	counts := map[string]int{}
	var order []string
	paths := map[string][]string{}
	for _, ds := range datasets {
		p := FlowPath(ds.Runs)
		k := strings.Join(p, ">")
		if _, ok := counts[k]; !ok {
			order = append(order, k)
			paths[k] = p
		}
		counts[k]++
	}
	best := ""
	bestN := 0
	for _, k := range order {
		if counts[k] > bestN {
			best, bestN = k, counts[k]
		}
	}
	if bestN == 0 {
		return []string{}
	}
	return paths[best]
}

// AgentStat aggregates one agent's calls (commits) and tokens across runs.
type AgentStat struct {
	Agent   string `json:"agent"`
	Runs    int    `json:"runs"`
	Commits int    `json:"commits"`
	Input   int64  `json:"input"`
	Output  int64  `json:"output"`
	Cached  int64  `json:"cached"`
	Total   int64  `json:"total"`
}

// AgentStats aggregates runs per agent, ordered by first appearance.
func AgentStats(runs []Run) []AgentStat {
	idx := map[string]int{}
	out := []AgentStat{}
	for _, r := range runs {
		i, ok := idx[r.Agent]
		if !ok {
			i = len(out)
			idx[r.Agent] = i
			out = append(out, AgentStat{Agent: r.Agent})
		}
		s := &out[i]
		s.Runs++
		s.Commits += r.Commits
		s.Input += r.Input
		s.Output += r.Output
		s.Cached += r.Cached
		s.Total += r.Total
	}
	return out
}

// Summary holds totals and per-run averages for one dataset.
type Summary struct {
	Name         string   `json:"name"`
	Source       string   `json:"source"`
	NoData       bool     `json:"noData,omitempty"`
	Runs         int      `json:"runs"`
	Commits      int      `json:"commits"`
	Input        int64    `json:"input"`
	Output       int64    `json:"output"`
	Cached       int64    `json:"cached"`
	Total        int64    `json:"total"`
	LinesChanged int      `json:"linesChanged"`
	TokenToCode  *float64 `json:"tokenToCode"`
	AvgTotal     *float64 `json:"avgTotalPerRun"`
	AvgCommits   *float64 `json:"avgCommitsPerRun"`
	Unattributed int      `json:"unattributed"`
	Untracked    int      `json:"untracked"`
	Skipped      int      `json:"skipped"`
	Flow         []string `json:"flow"`
}

// Summarize computes totals and per-run averages for ds.
func Summarize(ds Dataset) Summary {
	s := Summary{Name: ds.Name, Source: ds.Source, Runs: len(ds.Runs), Unattributed: ds.Unattributed,
		Untracked: len(ds.Untracked), Skipped: ds.Skipped, Flow: FlowPath(ds.Runs)}
	for _, r := range ds.Runs {
		s.Commits += r.Commits
		s.Input += r.Input
		s.Output += r.Output
		s.Cached += r.Cached
		s.Total += r.Total
		s.LinesChanged += r.LinesAdded + r.LinesRemoved
	}
	if s.LinesChanged > 0 {
		v := float64(s.Output) / float64(s.LinesChanged)
		s.TokenToCode = &v
	}
	if s.Runs > 0 {
		a, c := float64(s.Total)/float64(s.Runs), float64(s.Commits)/float64(s.Runs)
		s.AvgTotal, s.AvgCommits = &a, &c
	}
	return s
}

// MetricNames lists the compared metrics in display order.
var MetricNames = []string{"runs", "commits", "total", "output", "linesChanged", "tokenToCode", "avgTotalPerRun", "avgCommitsPerRun"}

func metricValue(s Summary, name string) *float64 {
	if s.NoData {
		return nil
	}
	f := func(v float64) *float64 { return &v }
	switch name {
	case "runs":
		return f(float64(s.Runs))
	case "commits":
		return f(float64(s.Commits))
	case "total":
		return f(float64(s.Total))
	case "output":
		return f(float64(s.Output))
	case "linesChanged":
		return f(float64(s.LinesChanged))
	case "tokenToCode":
		return s.TokenToCode
	case "avgTotalPerRun":
		return s.AvgTotal
	case "avgCommitsPerRun":
		return s.AvgCommits
	}
	return nil
}

// Cell is one metric value for one branch with its delta versus the baseline.
type Cell struct {
	Value    *float64 `json:"value"`
	Delta    *float64 `json:"delta"`
	DeltaPct *float64 `json:"deltaPct"`
}

// MetricRow is one metric across all compared branches.
type MetricRow struct {
	Metric string `json:"metric"`
	Cells  []Cell `json:"cells"`
}

// CompareResult is the N-way comparison of datasets against a baseline.
type CompareResult struct {
	Baseline    string         `json:"baseline"`
	Branches    []Summary      `json:"branches"`
	Metrics     []MetricRow    `json:"metrics"`
	Transitions [][]Transition `json:"transitions"`
}

// Compare builds per-metric values per dataset plus absolute and percent deltas versus
// the baseline (a dataset name; empty or unknown selects the first). Percent delta is
// nil when the baseline value is missing or zero.
func Compare(datasets []Dataset, baseline string) CompareResult {
	res := CompareResult{Branches: []Summary{}, Metrics: []MetricRow{}, Transitions: [][]Transition{}}
	bi := 0
	for i, ds := range datasets {
		res.Branches = append(res.Branches, Summarize(ds))
		res.Transitions = append(res.Transitions, Transitions(ds.Runs))
		if ds.Name == baseline {
			bi = i
		}
	}
	if len(datasets) > 0 {
		res.Baseline = datasets[bi].Name
	}
	for _, m := range MetricNames {
		row := MetricRow{Metric: m}
		base := metricValue(res.Branches[bi], m)
		for i, s := range res.Branches {
			c := Cell{Value: metricValue(s, m)}
			if i != bi && c.Value != nil && base != nil {
				d := *c.Value - *base
				c.Delta = &d
				if *base != 0 {
					p := d / *base * 100
					c.DeltaPct = &p
				}
			}
			row.Cells = append(row.Cells, c)
		}
		res.Metrics = append(res.Metrics, row)
	}
	return res
}

// NoDataSummary is the placeholder column for a selection that resolves to nothing.
func NoDataSummary(name string) Summary {
	return Summary{Name: name, NoData: true, Flow: []string{}}
}
