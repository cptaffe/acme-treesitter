package treesitter

import (
	"strings"

	"github.com/cptaffe/acme-styles/layer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// computeHighlights parses src with lang's grammar, runs the highlight query,
// applies any injection sub-grammars, and returns a slice of layer.Entry
// values (rune-offset based) ready for an acme-styles layer.
//
// "First capture wins": for a given byte position, whichever pattern appears
// earliest in the query file claims that position.  Injected language captures
// fill positions left unclaimed by the primary highlight pass.
func computeHighlights(lang *Language, src []byte) []layer.Entry {
	if lang == nil || lang.query == nil || len(src) == 0 {
		return nil
	}

	stylePerByte := make([]byte, len(src))
	highlightInto(lang, src, 0, stylePerByte)
	return compressToEntries(stylePerByte, src)
}

// highlightInto runs the primary highlight pass for lang over src, writing
// style indices into stylePerByte starting at byteOffset.  It then runs
// lang's injection query and recurses into each matched sub-language,
// filling positions left unclaimed by the primary pass.
func highlightInto(lang *Language, src []byte, byteOffset int, stylePerByte []byte) {
	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang.lang)

	tree := parser.Parse(src, nil)
	defer tree.Close()

	// Primary highlight pass.
	if lang.query != nil {
		qc := tree_sitter.NewQueryCursor()
		defer qc.Close()
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
			idx := lookupCaptureIdx(captureNames[cap.Index])
			if idx == 0 {
				continue
			}
			start := byteOffset + int(cap.Node.StartByte())
			end := byteOffset + int(cap.Node.EndByte())
			applyCapture(stylePerByte, start, end, idx)
		}
	}

	// Injection pass.
	if lang.injection != nil {
		applyInjections(lang.injection, tree.RootNode(), src, byteOffset, stylePerByte)
	}
}

// applyInjections runs lang's injection query against root, then for each
// matched (language, content) pair parses the content bytes with the
// appropriate sub-grammar and writes highlights into stylePerByte.
//
// Supported injection patterns:
//   - @injection.language capture: language name read from node text
//   - #set! injection.language "name": static language name
//   - @injection.content capture: byte range to parse
//
// Unsupported/unknown languages are silently skipped.
// The #set! injection.combined and injection.include-children predicates
// are not yet implemented.
func applyInjections(injQuery *tree_sitter.Query, root *tree_sitter.Node, src []byte, byteOffset int, stylePerByte []byte) {
	qc := tree_sitter.NewQueryCursor()
	defer qc.Close()

	captureNames := injQuery.CaptureNames()
	matches := qc.Matches(injQuery, root, src)

	for match := matches.Next(); match != nil; match = matches.Next() {
		// Collect static language from #set! injection.language "..." predicate.
		var staticLang string
		for _, prop := range injQuery.PropertySettings(uint(match.PatternIndex)) {
			if prop.Key == "injection.language" && prop.Value != nil {
				staticLang = *prop.Value
			}
		}

		// Collect captures: @injection.language and @injection.content.
		var langText string
		contentStart, contentEnd := -1, -1
		for _, cap := range match.Captures {
			if int(cap.Index) >= len(captureNames) {
				continue
			}
			switch captureNames[cap.Index] {
			case "injection.language":
				langText = string(src[cap.Node.StartByte():cap.Node.EndByte()])
			case "injection.content":
				contentStart = int(cap.Node.StartByte())
				contentEnd = int(cap.Node.EndByte())
			}
		}

		if contentStart < 0 || contentStart >= contentEnd {
			continue
		}

		langID := staticLang
		if langID == "" {
			langID = strings.ToLower(strings.TrimSpace(langText))
		}
		if langID == "" {
			continue
		}

		subLang := langByID(langID)
		if subLang == nil || subLang.query == nil {
			continue
		}

		content := src[contentStart:contentEnd]
		highlightInto(subLang, content, byteOffset+contentStart, stylePerByte)
	}
}

// stylePerByte[i] = canonicalTable index (≥1) for byte i; 0 = unclaimed.
// We use uint8 — canonicalTable has ≤ 20 entries.

// applyCapture marks bytes [start, end) in stylePerByte with idx,
// but only where the slot is still 0 ("first match wins").
func applyCapture(stylePerByte []byte, start, end, idx int) {
	if idx == 0 {
		return
	}
	for i := start; i < end && i < len(stylePerByte); i++ {
		if stylePerByte[i] == 0 {
			stylePerByte[i] = byte(idx)
		}
	}
}

// compressToEntries converts a per-byte style-index array (stylePerByte[i] is
// an index into canonicalTable; 0 = unstyled) into a slice of layer.Entry
// values using rune offsets (Start inclusive, End exclusive).
func compressToEntries(stylePerByte []byte, src []byte) []layer.Entry {
	var entries []layer.Entry
	byteOff := 0
	runeOff := 0
	curIdx := 0
	spanStart := 0

	for byteOff < len(src) {
		_, size := runeSize(src[byteOff:])

		idx := int(stylePerByte[byteOff])
		if idx != curIdx {
			if curIdx != 0 {
				entries = append(entries, layer.Entry{
					Name:  canonicalTable[curIdx],
					Start: spanStart,
					End:   runeOff,
				})
			}
			curIdx = idx
			spanStart = runeOff
		}

		byteOff += size
		runeOff++
	}
	if curIdx != 0 {
		entries = append(entries, layer.Entry{
			Name:  canonicalTable[curIdx],
			Start: spanStart,
			End:   runeOff,
		})
	}
	return entries
}

// runeSize returns the number of bytes in the first UTF-8 rune in b,
// or 1 for invalid/empty sequences (matching unicode/utf8.DecodeRune).
func runeSize(b []byte) (rune, int) {
	if len(b) == 0 {
		return 0, 1
	}
	r := rune(b[0])
	if r < 0x80 {
		return r, 1
	}
	// Multi-byte: find length from leading byte.
	var size int
	switch {
	case r&0xE0 == 0xC0:
		size = 2
	case r&0xF0 == 0xE0:
		size = 3
	case r&0xF8 == 0xF0:
		size = 4
	default:
		return r, 1 // invalid
	}
	if size > len(b) {
		return r, 1
	}
	return r, size
}
