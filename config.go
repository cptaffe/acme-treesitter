package treesitter

import (
	"fmt"
	"regexp"

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
