package treesitter

import "testing"

// TestMermaidInjection verifies that a ```mermaid fenced code block inside a
// markdown document is highlighted with the mermaid grammar via the injection
// pass.  It also guards against the regression where the primary markdown pass
// claimed the whole fenced_code_block and blocked all fence injections.
func TestMermaidInjection(t *testing.T) {
	src := []byte("# Title\n\n```mermaid\nsequenceDiagram\n  Alice->>John: Hi\n```\n")
	entries := computeHighlights(langByID("markdown"), src)

	runes := []rune(string(src))
	var gotKeyword, gotArrow bool
	for _, e := range entries {
		text := string(runes[e.Start:e.End])
		if text == "sequenceDiagram" && e.Name == "keyword" {
			gotKeyword = true
		}
		if text == "->>" && e.Name == "operator" {
			gotArrow = true
		}
	}
	if !gotKeyword {
		t.Error("mermaid keyword 'sequenceDiagram' not highlighted inside markdown fence")
	}
	if !gotArrow {
		t.Error("mermaid arrow '->>' not highlighted inside markdown fence")
	}
}
