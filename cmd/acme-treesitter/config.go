package main

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Config is the top-level structure of ~/lib/acme-treesitter/config.yaml.
type Config struct {
	// StyleFile is unused; kept for backward compatibility with existing
	// config files.  The palette is managed by acme-styles.
	StyleFile string `yaml:"style_file"`

	// FilenameHandlers maps filename patterns to grammar language IDs.
	// Evaluated in order; first match wins.  Patterns are Go regular
	// expressions; the same regexes used in acme-lsp's FilenameHandlers work
	// here unchanged.
	FilenameHandlers []FilenameHandler `yaml:"filename_handlers"`
}

// FilenameHandler associates a filename regex pattern with a grammar language ID.
type FilenameHandler struct {
	Pattern    string `yaml:"pattern"`
	LanguageID string `yaml:"language_id"`
}

// runtimeHandler is a compiled FilenameHandler, ready for matching.
type runtimeHandler struct {
	re   *regexp.Regexp
	lang *Language // nil if LanguageID is unsupported
}

// LoadConfig reads path and returns the parsed Config.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

// compileHandlers pre-compiles the FilenameHandler regexes from cfg.
// Language grammars must have been initialised (initLanguages) first.
// Handlers whose regex is invalid are skipped with a warning.
func compileHandlers(cfg *Config) ([]runtimeHandler, error) {
	out := make([]runtimeHandler, 0, len(cfg.FilenameHandlers))
	for _, fh := range cfg.FilenameHandlers {
		re, err := regexp.Compile(fh.Pattern)
		if err != nil {
			return nil, fmt.Errorf("FilenameHandler pattern %q: %w", fh.Pattern, err)
		}
		out = append(out, runtimeHandler{
			re:   re,
			lang: langByID(fh.LanguageID),
		})
	}
	return out, nil
}

// detectLanguageForName returns the Language for filename name using the
// compiled handler list, or nil if no pattern matches or the matched language
// ID has no registered grammar.
func detectLanguageForName(handlers []runtimeHandler, name string) *Language {
	for _, h := range handlers {
		if h.re.MatchString(name) {
			return h.lang // may be nil for unsupported LanguageID
		}
	}
	return nil
}
