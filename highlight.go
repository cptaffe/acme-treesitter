package treesitter

import (
	"github.com/cptaffe/acme-styles/layer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// computeHighlights parses src with lang's grammar, runs the highlight query,
// and returns a slice of layer.Entry values (rune-offset based) ready for an
// acme-styles layer.
//
// "First capture wins": for a given byte position, whichever pattern appears
// earliest in the query file claims that position.  Later catch-all patterns
// (e.g. @variable) therefore do not overwrite specific ones (e.g. @function).
func computeHighlights(lang *Language, src []byte) []layer.Entry {
	if lang == nil || lang.query == nil || len(src) == 0 {
		return nil
	}

	// Each goroutine needs its own Parser and QueryCursor.
	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang.lang)

	tree := parser.Parse(src, nil)
	defer tree.Close()

	qc := tree_sitter.NewQueryCursor()
	defer qc.Close()

	// stylePerByte[i] = canonicalTable index (≥1) for byte i; 0 = unclaimed.
	// We use uint8 — canonicalTable has ≤ 10 entries.
	stylePerByte := make([]byte, len(src))

	captureNames := lang.query.CaptureNames()
	captures := qc.Captures(lang.query, tree.RootNode(), src)

	for match, captureIdx := captures.Next(); match != nil; match, captureIdx = captures.Next() {
		if int(captureIdx) >= len(match.Captures) {
			continue
		}
		cap := match.Captures[captureIdx]
		if int(cap.Index) >= len(captureNames) {
			continue
		}
		capName := captureNames[cap.Index]
		idx := lookupCaptureIdx(capName)
		if idx == 0 {
			continue
		}
		start := int(cap.Node.StartByte())
		end := int(cap.Node.EndByte())
		applyCapture(stylePerByte, start, end, idx)
	}

	return compressToEntries(stylePerByte, src)
}
