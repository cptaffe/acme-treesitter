package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// StyleEntry is a (style-index, start-rune, end-rune) triple ready for
// writing to an acme-styles layer or acme's native style file.
// Start is inclusive, End is exclusive — matching acme's event coordinates.
type StyleEntry struct {
	StyleIdx int
	Start    int
	End      int // exclusive
}

// StyleMap maps style name → index as defined in the acme styles file.
// Index 0 is "default" (transparent).
type StyleMap map[string]int

// LoadStyleFile reads an acme styles file and returns a name→index map.
// Lines beginning with '#' are ignored.
func LoadStyleFile(path string) (StyleMap, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sm := make(StyleMap)
	idx := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		sm[fields[0]] = idx
		idx++
	}
	return sm, sc.Err()
}

// lookupCapture converts a tree-sitter capture name (e.g. "function.method")
// to a style index using a hierarchical fallback: "function.method" →
// "function" → 0 (not found).  Index 0 means "skip this capture".
func lookupCapture(sm StyleMap, captureName string) int {
	name := strings.TrimPrefix(captureName, "@")
	for {
		if idx, ok := sm[name]; ok {
			return idx
		}
		dot := strings.LastIndex(name, ".")
		if dot < 0 {
			return 0
		}
		name = name[:dot]
	}
}

// compressToEntries converts a per-byte style array (stylePerByte[i] = style
// index for byte i, 0 = unstyled) into a slice of StyleEntry values using
// rune offsets (Start inclusive, End exclusive), suitable for writing to an
// acme-styles layer or acme's native style file.
func compressToEntries(stylePerByte []byte, src []byte) []StyleEntry {
	var entries []StyleEntry
	byteOff := 0
	runeOff := 0
	curStyle := 0
	spanStart := 0

	for byteOff < len(src) {
		r := src[byteOff]
		var size int
		switch {
		case r < 0x80:
			size = 1
		case r < 0xE0:
			size = 2
		case r < 0xF0:
			size = 3
		default:
			size = 4
		}
		if byteOff+size > len(src) {
			size = 1 // guard against truncated input
		}

		style := int(stylePerByte[byteOff])
		if style != curStyle {
			if curStyle != 0 {
				entries = append(entries, StyleEntry{
					StyleIdx: curStyle,
					Start:    spanStart,
					End:      runeOff,
				})
			}
			curStyle = style
			spanStart = runeOff
		}

		byteOff += size
		runeOff++
	}
	if curStyle != 0 {
		entries = append(entries, StyleEntry{
			StyleIdx: curStyle,
			Start:    spanStart,
			End:      runeOff,
		})
	}
	return entries
}

// applyCapture marks bytes [start, end) in stylePerByte with styleIdx,
// but only where the slot is still 0 ("first match wins").
func applyCapture(stylePerByte []byte, start, end, styleIdx int) {
	if styleIdx == 0 {
		return
	}
	for i := start; i < end && i < len(stylePerByte); i++ {
		if stylePerByte[i] == 0 {
			stylePerByte[i] = byte(styleIdx)
		}
	}
}

// styleEntriesToText formats style entries as "idx start end\n" lines,
// one per entry, ready for writing to an acme-styles layer style file or
// acme's native <winid>/style file.
func styleEntriesToText(entries []StyleEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&sb, "%d %d %d\n", e.StyleIdx, e.Start, e.End)
	}
	return sb.String()
}
