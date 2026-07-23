package treesitter

import (
	_ "embed"
	"log"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_bash "github.com/tree-sitter/tree-sitter-bash/bindings/go"
	tree_sitter_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tree_sitter_js "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tree_sitter_scala "github.com/tree-sitter/tree-sitter-scala/bindings/go"
	tree_sitter_toml "github.com/tree-sitter-grammars/tree-sitter-toml/bindings/go"
	tree_sitter_markdown "github.com/cptaffe/acme-treesitter/markdown"
	tree_sitter_markdown_inline "github.com/cptaffe/acme-treesitter/markdown_inline"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
	tree_sitter_yaml "github.com/tree-sitter-grammars/tree-sitter-yaml/bindings/go"
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

//go:embed queries/java.scm
var javaHighlights string

//go:embed queries/scala.scm
var scalaHighlights string

//go:embed queries/markdown.scm
var markdownHighlights string

//go:embed queries/markdown_inline.scm
var markdownInlineHighlights string

//go:embed queries/typescript.scm
var typescriptHighlights string

//go:embed queries/yaml.scm
var yamlHighlights string

//go:embed queries/json.scm
var jsonHighlights string

//go:embed queries/toml.scm
var tomlHighlights string

//go:embed queries/markdown.injections.scm
var markdownInjections string

//go:embed queries/rust.injections.scm
var rustInjections string

// Language bundles a compiled tree-sitter Language pointer and pre-compiled
// highlight and injection queries.  All fields are safe to share across
// goroutines (read-only after init).
type Language struct {
	Name      string
	lang      *tree_sitter.Language
	query     *tree_sitter.Query // highlight query; nil if compilation failed
	injection *tree_sitter.Query // injection query; nil if none
}

// langByName maps language_id strings → *Language.
// Populated once by init; looked up via langByID.
var langByName map[string]*Language

// init compiles all language grammars and their highlight and injection queries.
// Grammars whose highlight query fails are registered without a query (files
// open without highlighting rather than crashing).
func init() {
	type spec struct {
		id        string
		lang      *tree_sitter.Language
		highlight string
		injection string // empty = no injection query
	}

	specs := []spec{
		{"go", tree_sitter.NewLanguage(tree_sitter_go.Language()), goHighlights, ""},
		{"c", tree_sitter.NewLanguage(tree_sitter_c.Language()), cHighlights, ""},
		{"cpp", tree_sitter.NewLanguage(tree_sitter_c.Language()), cHighlights, ""}, // fallback: C grammar for C++
		{"python", tree_sitter.NewLanguage(tree_sitter_python.Language()), pythonHighlights, ""},
		{"rust", tree_sitter.NewLanguage(tree_sitter_rust.Language()), rustHighlights, rustInjections},
		{"javascript", tree_sitter.NewLanguage(tree_sitter_js.Language()), jsHighlights, ""},
		{"bash", tree_sitter.NewLanguage(tree_sitter_bash.Language()), bashHighlights, ""},
		{"java", tree_sitter.NewLanguage(tree_sitter_java.Language()), javaHighlights, ""},
		{"scala", tree_sitter.NewLanguage(tree_sitter_scala.Language()), scalaHighlights, ""},
		{"markdown", tree_sitter.NewLanguage(tree_sitter_markdown.Language()), markdownHighlights, markdownInjections},
		{"markdown_inline", tree_sitter.NewLanguage(tree_sitter_markdown_inline.Language()), markdownInlineHighlights, ""},
		// The TypeScript and TSX grammars are supersets of JavaScript and
		// share its node names, but typescript.scm only holds the
		// TypeScript-specific additions (types, modifiers).  Upstream
		// (nvim-treesitter, Helix) layers it on the JavaScript query via
		// "; inherits: javascript"; this framework has no inherit support, so
		// prepend the JavaScript base query explicitly.  Without this, tsx/ts
		// files highlight only type annotations and leave all JavaScript syntax
		// (functions, strings, comments, JSX, ...) unstyled.
		{"typescript", tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript()), jsHighlights + "\n" + typescriptHighlights, ""},
		{"tsx", tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTSX()), jsHighlights + "\n" + typescriptHighlights, ""},
		{"yaml", tree_sitter.NewLanguage(tree_sitter_yaml.Language()), yamlHighlights, ""},
		{"json", tree_sitter.NewLanguage(tree_sitter_json.Language()), jsonHighlights, ""},
		{"toml", tree_sitter.NewLanguage(tree_sitter_toml.Language()), tomlHighlights, ""},
	}

	langByName = make(map[string]*Language, len(specs))
	for _, s := range specs {
		l := &Language{Name: s.id, lang: s.lang}

		q, qerr := tree_sitter.NewQuery(s.lang, s.highlight)
		if qerr != nil {
			log.Printf("lang %s: highlight query error at offset %d: %s", s.id, qerr.Offset, qerr.Message)
		} else {
			l.query = q
		}

		if s.injection != "" {
			iq, iqerr := tree_sitter.NewQuery(s.lang, s.injection)
			if iqerr != nil {
				log.Printf("lang %s: injection query error at offset %d: %s", s.id, iqerr.Offset, iqerr.Message)
			} else {
				l.injection = iq
			}
		}

		langByName[s.id] = l
	}
}

// langByID returns the Language for the given language_id, or nil if unknown.
func langByID(id string) *Language {
	return langByName[id]
}
