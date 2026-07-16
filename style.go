package treesitter

import (
	_ "embed"
	"strings"
)

//go:embed token_names.txt
var tokenNamesData string

// canonicalTable is the ordered list of short palette names derived from
// token_names.txt.  Index 0 is the "no style" sentinel.
var canonicalTable []string

// canonicalIndex maps capture name stems and palette names to indices in
// canonicalTable.  Populated by init() from token_names.txt.
var canonicalIndex map[string]int

func init() {
	table := []string{""} // index 0 = unstyled
	paletteIdx := make(map[string]int) // palette name → index (dedup)
	index := make(map[string]int)

	for _, line := range strings.Split(tokenNamesData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		palette, source := fields[0], fields[1]

		idx, ok := paletteIdx[palette]
		if !ok {
			idx = len(table)
			table = append(table, palette)
			paletteIdx[palette] = idx
			index[palette] = idx // palette name maps to itself
		}
		index[source] = idx
	}

	canonicalTable = table
	canonicalIndex = index
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

