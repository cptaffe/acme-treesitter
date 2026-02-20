package treesitter

import (
	_ "embed"
	"log"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_bash "github.com/tree-sitter/tree-sitter-bash/bindings/go"
	tree_sitter_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_js "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
)

//go:embed queries/go.scm
var goHighlights string

//go:embed queries/c.scm
var cHighlights string

//go:embed queries/python.scm
var pythonHighlights string

//go:embed queries/rust.scm
var rustHighlights string

//go:embed queries/javascript.scm
var jsHighlights string

//go:embed queries/bash.scm
var bashHighlights string

// Language bundles a compiled tree-sitter Language pointer and a pre-compiled
// Query.  Both are safe to share across goroutines (read-only after init).
type Language struct {
	Name  string
	lang  *tree_sitter.Language
	query *tree_sitter.Query // nil if query compilation failed
}

// langByName maps language_id strings → *Language.
// Populated once by initLanguages; looked up via langByID.
var (
	langOnce   sync.Once
	langByName map[string]*Language
)

func init() {
	initLanguages()
}

// initLanguages compiles all language grammars and their highlight queries.
// Grammars whose query fails to parse are registered without a query (files
// open without highlighting rather than crashing).
func initLanguages() {
	langOnce.Do(func() {
		specs := []struct {
			id    string // matches language_id values used in config.yaml
			lang  *tree_sitter.Language
			query string
		}{
			{"go", tree_sitter.NewLanguage(tree_sitter_go.Language()), goHighlights},
			{"c", tree_sitter.NewLanguage(tree_sitter_c.Language()), cHighlights},
			{"cpp", tree_sitter.NewLanguage(tree_sitter_c.Language()), cHighlights}, // fallback: use C grammar for C++ until tree-sitter-cpp is added
			{"python", tree_sitter.NewLanguage(tree_sitter_python.Language()), pythonHighlights},
			{"rust", tree_sitter.NewLanguage(tree_sitter_rust.Language()), rustHighlights},
			{"javascript", tree_sitter.NewLanguage(tree_sitter_js.Language()), jsHighlights},
			{"bash", tree_sitter.NewLanguage(tree_sitter_bash.Language()), bashHighlights},
		}

		langByName = make(map[string]*Language, len(specs))
		for _, s := range specs {
			l := &Language{Name: s.id, lang: s.lang}
			q, qerr := tree_sitter.NewQuery(s.lang, s.query)
			if qerr != nil {
				log.Printf("lang %s: query error at offset %d: %s", s.id, qerr.Offset, qerr.Message)
				// Register without a query — windows open without highlighting.
			} else {
				l.query = q
				// q is never closed; it lives for the process lifetime and is
				// shared (read-only) across all goroutines.
			}
			langByName[s.id] = l
		}
	})
}

// langByID returns the Language for the given language_id, or nil if unknown.
// initLanguages must have been called first.
func langByID(id string) *Language {
	return langByName[id]
}
