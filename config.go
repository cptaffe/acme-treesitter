package treesitter

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cptaffe/acme-treesitter/config"
)

// Handler is a compiled FilenameHandler, ready for matching.
type Handler struct {
	re   *regexp.Regexp
	lang *Language // nil if LanguageID is unsupported
}

// CompileHandlers pre-compiles the FilenameHandler regexes from cfg.
// Handlers whose regex is invalid are returned as an error.
func CompileHandlers(cfg *config.Config) ([]Handler, error) {
	out := make([]Handler, 0, len(cfg.FilenameHandlers))
	for _, fh := range cfg.FilenameHandlers {
		re, err := regexp.Compile(fh.Pattern)
		if err != nil {
			return nil, fmt.Errorf("FilenameHandler pattern %q: %w", fh.Pattern, err)
		}
		out = append(out, Handler{
			re:   re,
			lang: langByID(fh.LanguageID),
		})
	}
	return out, nil
}

// detectLanguage returns the Language for filename name using the compiled
// handler list, or nil if no pattern matches or the matched language ID has
// no registered grammar.
func detectLanguage(handlers []Handler, name string) *Language {
	for _, h := range handlers {
		if h.re.MatchString(name) {
			return h.lang
		}
	}
	return nil
}

// shebangs maps interpreter base-names to language IDs.
// Version suffixes (python3.11, node20, …) are stripped before lookup.
var shebangs = map[string]string{
	// Shell
	"ash":  "bash",
	"bash": "bash",
	"dash": "bash",
	"fish": "bash",
	"ksh":  "bash",
	"sh":   "bash",
	"zsh":  "bash",
	// Python
	"python":  "python",
	"python2": "python",
	"python3": "python",
	// JavaScript / TypeScript
	"bun":     "javascript",
	"deno":    "javascript",
	"node":    "javascript",
	"nodejs":  "javascript",
	"ts-node": "javascript",
	// Java
	"java":  "java",
	"jbang": "java",
	// Scala
	"amm":    "scala", // Ammonite script runner
	"scala":  "scala",
	"scala3": "scala",
	// Rust
	"rust-script": "rust",
}

// detectByShebang parses the first line of a file and returns a Language if
// it starts with a recognized #! interpreter line, or nil otherwise.
func detectByShebang(firstLine string) *Language {
	interp := shebanInterpreter(firstLine)
	if interp == "" {
		return nil
	}
	id := langIDForInterpreter(interp)
	return langByID(id) // nil if id=="" or grammar not registered
}

// shebanInterpreter extracts the interpreter base-name from a shebang line.
//
// It handles the common forms:
//
//	#!/bin/bash
//	#!/usr/bin/env python3
//	#!/usr/bin/env -S scala -classpath lib   (env flags are skipped)
func shebanInterpreter(line string) string {
	if !strings.HasPrefix(line, "#!") {
		return ""
	}
	fields := strings.Fields(line[2:])
	if len(fields) == 0 {
		return ""
	}
	base := filepath.Base(fields[0])
	if base == "env" {
		// Skip env flags/options (anything starting with '-') and return the
		// first plain argument, which is the actual interpreter.
		for _, f := range fields[1:] {
			if !strings.HasPrefix(f, "-") {
				return filepath.Base(f)
			}
		}
		return ""
	}
	return base
}

// langIDForInterpreter maps an interpreter base-name to a language ID.
// It first tries an exact match, then strips trailing version characters
// (digits and dots) and tries again, so "python3.11" → "python".
func langIDForInterpreter(name string) string {
	if id, ok := shebangs[name]; ok {
		return id
	}
	// Strip trailing version suffix: python3.11 → python, node20 → node.
	stripped := strings.TrimRightFunc(name, func(r rune) bool {
		return r == '.' || (r >= '0' && r <= '9')
	})
	if stripped != "" && stripped != name {
		if id, ok := shebangs[stripped]; ok {
			return id
		}
	}
	return ""
}
