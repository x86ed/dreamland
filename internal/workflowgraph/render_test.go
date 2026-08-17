package workflowgraph

import "strings"

import "testing"

func TestRenderCursor(t *testing.T) {
	in := renderInputs{
		Agent: &AgentNode{
			ID:              "hypnos",
			Description:     "Shapes dreams.",
			Role:            "worker",
			InstructionBody: "Do the work.",
		},
		RoutesTo: []string{"morpheus"},
	}

	out := renderCursor(in)

	if !strings.HasPrefix(out, "---\ndescription: Shapes dreams.\nalwaysApply: false\n---\n\n") {
		t.Fatalf("unexpected frontmatter, got:\n%s", out)
	}
	if !strings.Contains(out, "Do the work.") {
		t.Errorf("expected body in output, got:\n%s", out)
	}
	if !strings.Contains(out, "hand off directly to `morpheus`") {
		t.Errorf("expected hand-off sentence in output, got:\n%s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("expected trailing newline, got:\n%q", out)
	}
}

func TestRenderCodex(t *testing.T) {
	in := renderInputs{
		Agent: &AgentNode{
			ID:              "hypnos",
			Description:     "Shapes dreams.",
			Role:            "worker",
			InstructionBody: "Do the work.",
		},
	}

	out := renderCodex(in)

	if !strings.Contains(out, `name = "hypnos"`) {
		t.Errorf("expected name field, got:\n%s", out)
	}
	if !strings.Contains(out, `description = "Shapes dreams."`) {
		t.Errorf("expected description field, got:\n%s", out)
	}
	if !strings.Contains(out, "developer_instructions = \"\"\"\nDo the work.\n\"\"\"") {
		t.Errorf("expected developer_instructions block, got:\n%s", out)
	}
}

func TestRenderKiro(t *testing.T) {
	in := renderInputs{
		Agent: &AgentNode{
			ID:              "hypnos-serve",
			Description:     "Shapes dreams.",
			Role:            "worker",
			InstructionBody: "Do the work.",
		},
	}

	out := renderKiro(in)

	if !strings.HasPrefix(out, "---\nname: hypnos-serve\ndescription: Shapes dreams.\ninclusion: always\n---\n\n# Hypnos Serve\n\n") {
		t.Fatalf("unexpected header, got:\n%s", out)
	}
	if !strings.Contains(out, "Do the work.") {
		t.Errorf("expected body in output, got:\n%s", out)
	}
}

func TestRenderAntigravity(t *testing.T) {
	in := renderInputs{
		Agent: &AgentNode{
			ID:              "hypnos",
			Description:     "Shapes dreams.",
			Role:            "worker",
			InstructionBody: "Do the work.",
		},
	}

	out := renderAntigravity(in)

	if !strings.HasPrefix(out, "---\nname: hypnos\ndescription: Shapes dreams.\n---\n\n") {
		t.Fatalf("unexpected frontmatter, got:\n%s", out)
	}
	if !strings.Contains(out, "Do the work.") {
		t.Errorf("expected body in output, got:\n%s", out)
	}
}

func TestTitleCase(t *testing.T) {
	cases := map[string]string{
		"hypnos":            "Hypnos",
		"hypnos-serve":      "Hypnos Serve",
		"multi-word-agent":  "Multi Word Agent",
		"":                  "",
		"leading--doubledash": "Leading  Doubledash",
	}
	for in, want := range cases {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}
