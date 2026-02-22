package treesitter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"9fans.net/go/acme"
	"github.com/cptaffe/acme-styles/layer"
	"github.com/cptaffe/acme-treesitter/logger"
	"go.uber.org/zap"
)

const layerName = "treesitter"

const debounceDuration = 200 * time.Millisecond

// errWindowClosed is returned by runWindowOnce when the window's edit log
// reaches EOF cleanly — i.e. the user closed the window.
var errWindowClosed = errors.New("window closed")

// RunWindow is the per-window entry point.  It detects the file's language
// (filename patterns first, shebang fallback) and runs one highlight session
// via runWindowOnce.  It exits when the window is closed, the session fails,
// or ctx is cancelled.
func RunWindow(ctx context.Context, id int, name string, handlers []Handler) {
	ctx = logger.NewContext(ctx, logger.L(ctx).With(zap.Int("window", id), zap.String("name", name)))
	log := logger.L(ctx)

	lang := detectLang(ctx, id, name, handlers)
	if lang == nil {
		log.Debug("no handler matched")
		return
	}
	log.Debug("matched language", zap.String("lang", lang.Name))

	err := runWindowOnce(ctx, id, lang)
	switch {
	case err == nil || errors.Is(err, errWindowClosed):
		log.Debug("window closed")
	case ctx.Err() != nil:
		// context cancelled; shutting down
	default:
		log.Warn("session error", zap.Error(err))
	}
}

// detectLang returns the Language for the given window, trying filename
// patterns first and falling back to shebang detection.  Returns nil if
// no pattern matches or the window is unavailable.
func detectLang(ctx context.Context, id int, name string, handlers []Handler) *Language {
	if lang := detectLanguage(handlers, name); lang != nil {
		return lang
	}
	// Shebang fallback — need an acme connection.
	w, err := acme.Open(id, nil)
	if err != nil {
		return nil
	}
	body, err := w.ReadBody()
	w.CloseFiles()
	if err != nil {
		return nil
	}
	return detectByShebang(firstLine(body))
}

// firstLine returns the content of body up to (but not including) the first
// newline, or the whole body if there is no newline.
func firstLine(body []byte) string {
	if i := bytes.IndexByte(body, '\n'); i >= 0 {
		return string(body[:i])
	}
	return string(body)
}

// runWindowOnce performs one complete highlight session for a window:
//   - opens an acme-styles compositor layer,
//   - opens the window via the shared acme connection,
//   - does an initial parse + highlight,
//   - watches the per-window edit log and re-highlights after edits.
//
// It returns errWindowClosed on clean log EOF, ctx.Err() if the context is
// cancelled, or another error for transient failures the caller should retry.
func runWindowOnce(ctx context.Context, id int, lang *Language) error {
	log := logger.L(ctx)

	sl, err := layer.Open(id, layerName)
	if err != nil {
		return fmt.Errorf("open layer: %w", err)
	}
	log.Debug("allocated layer", zap.Int("layerID", sl.LayerID))
	defer sl.Delete()

	// acme.Open uses the package-level shared connection (defaultFsys), so all
	// window goroutines share a single OS-level socket to acme rather than
	// each dialling their own.
	w, err := acme.Open(id, nil)
	if err != nil {
		return fmt.Errorf("open acme win: %w", err)
	}

	if err := doHighlight(ctx, lang, sl, w); err != nil {
		w.CloseFiles()
		return fmt.Errorf("initial highlight: %w", err)
	}
	log.Debug("initial highlight ok")

	timer := time.NewTimer(debounceDuration)
	timer.Stop()
	pending := false

	// lines carries edit notifications (I/D events) from the scanner goroutine.
	// scanResult carries the exit reason: nil = clean EOF (window closed), else error.
	// goroutineExited is closed after the goroutine writes to scanResult.
	lines := make(chan struct{}, 32)
	scanResult := make(chan error, 1) // buffered so the goroutine never blocks
	goroutineExited := make(chan struct{})

	go func() {
		defer close(goroutineExited)
		for {
			e, err := w.ReadLog()
			if err != nil {
				scanResult <- err
				return
			}
			if e.Op == 'I' || e.Op == 'D' {
				select {
				case lines <- struct{}{}:
				default:
				}
			}
		}
	}()

	defer func() {
		w.CloseFiles()    // closes the log fid, unblocking ReadLog in the goroutine
		<-goroutineExited // wait for it to finish
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-lines:
			if !pending {
				timer.Reset(debounceDuration)
				pending = true
			}

		case err := <-scanResult:
			if err == nil {
				return errWindowClosed
			}
			return err

		case <-timer.C:
			pending = false
			if err := doHighlight(ctx, lang, sl, w); err != nil {
				return fmt.Errorf("re-highlight: %w", err)
			}
		}
	}
}

// doHighlight reads the window body, parses it with tree-sitter, and writes
// the resulting highlight entries to sl.
func doHighlight(ctx context.Context, lang *Language, sl *layer.StyleLayer, w *acme.Win) error {
	log := logger.L(ctx)
	// ReadBody opens a fresh fid each time so reading always starts at offset 0.
	body, err := w.ReadBody()
	if err != nil {
		return err
	}
	entries := computeHighlights(lang, body)
	log.Debug("highlight entries computed", zap.Int("count", len(entries)))
	return sl.Apply(entries)
}
