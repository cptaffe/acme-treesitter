package treesitter

import (
	"strings"
	"unicode/utf8"

	"github.com/cptaffe/acme-styles/layer"
)

// canonicalTable is the ordered list of short palette names that
// acme-treesitter emits.  Index 0 is the "no style" sentinel.
// These names must match the palette entries in the master styles file.
var canonicalTable = []string{
	"", // 0 = unstyled
	"k", // keyword
	"c", // comment
	"s", // string
	"t", // type
	"n", // number
	"o", // operator
	"e", // error
	"f", // function
	"m", // macro
}

// canonicalIndex maps both short and long capture names to indices in
// canonicalTable.  The long names are used in the .scm query files;
// the short names are the emitted palette entry names.
var canonicalIndex = map[string]int{
	// short names (palette entries)
	"k": 1, "c": 2, "s": 3, "t": 4, "n": 5,
	"o": 6, "e": 7, "f": 8, "m": 9,
	// long names (tree-sitter capture name stems in .scm files)
	"keyword": 1, "comment": 2, "string": 3, "type": 4, "number": 5,
	"operator": 6, "error": 7, "function": 8, "macro": 9,
}

// lookupCaptureIdx converts a tree-sitter capture name (e.g. "@function.method")
// to a canonicalTable index using hierarchical fallback:
//
//	"function.method" → "function" → index 8 ("f")
//
// Index 0 means "skip this capture".  Callers retrieve the name via
// canonicalTable[idx].
func lookupCaptureIdx(captureName string) int {
	name := strings.TrimPrefix(captureName, "@")
	for {
		if idx, ok := canonicalIndex[name]; ok {
			return idx
		}
		dot := strings.LastIndex(name, ".")
		if dot < 0 {
			return 0
		}
		name = name[:dot]
	}
}

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
		_, size := utf8.DecodeRune(src[byteOff:])

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
