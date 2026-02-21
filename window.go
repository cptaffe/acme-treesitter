package treesitter

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"9fans.net/go/plan9"
	"9fans.net/go/plan9/client"
	"github.com/cptaffe/acme-styles/layer"
	"github.com/cptaffe/acme-treesitter/logger"
	"go.uber.org/zap"
)

const layerName = "treesitter"

const debounceDuration = 200 * time.Millisecond

// errWindowClosed is returned by runWindowOnce when the window's edit log
// reaches EOF cleanly — i.e. the user closed the window.
var errWindowClosed = errors.New("window closed")

// RunWindow is the per-window control loop.  It:
//
//  1. Detects the file's language (filename patterns, then shebang fallback).
//     If acme is temporarily unavailable for shebang detection it retries
//     with exponential backoff rather than giving up.
//  2. Enters a reconcile loop that calls runWindowOnce.  On any transient
//     error (compositor down, acme connection dropped, etc.) it waits with
//     exponential backoff and tries again.
//
// The loop exits only when the window is closed (clean EOF from the edit
// log) or ctx is cancelled.
func RunWindow(ctx context.Context, id int, name string, handlers []Handler) {
	ctx = logger.NewContext(ctx, logger.L(ctx).With(zap.Int("window", id), zap.String("name", name)))
	log := logger.L(ctx)

	lang := detectLang(ctx, id, name, handlers)
	if lang == nil {
		log.Debug("no handler matched")
		return
	}
	log.Debug("matched language", zap.String("lang", lang.Name))

	bo := backoff{base: 200 * time.Millisecond, cap: time.Minute}
	for {
		err := runWindowOnce(ctx, id, lang)
		switch {
		case err == nil || errors.Is(err, errWindowClosed):
			log.Debug("window closed")
			return
		case ctx.Err() != nil:
			return
		default:
			d := bo.next()
			log.Warn("session error, retrying", zap.Error(err), zap.Duration("in", d))
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return
			}
		}
	}
}

// detectLang returns the Language for the given window, trying filename
// patterns first and falling back to shebang detection.  Shebang detection
// requires reading the body from acme; if acme is temporarily unavailable
// it retries with backoff rather than returning nil immediately.
func detectLang(ctx context.Context, id int, name string, handlers []Handler) *Language {
	if lang := detectLanguage(handlers, name); lang != nil {
		return lang
	}
	// Shebang fallback — need an acme connection.  Retry if not ready yet.
	bo := backoff{base: 200 * time.Millisecond, cap: time.Minute}
	for {
		fs, err := client.MountService("acme")
		if err != nil {
			select {
			case <-time.After(bo.next()):
				continue
			case <-ctx.Done():
				return nil
			}
		}
		first, err := readFirstLine(id, fs)
		fs.Close()
		if err != nil {
			// Window probably gone; treat as no match.
			return nil
		}
		return detectByShebang(first) // nil if no recognised shebang
	}
}

// runWindowOnce performs one complete highlight session for a window:
//   - opens an acme-styles compositor layer,
//   - opens a fresh acme connection,
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

	fs, err := client.MountService("acme")
	if err != nil {
		return fmt.Errorf("mount acme: %w", err)
	}
	defer fs.Close()

	if err := doHighlight(ctx, id, lang, sl, fs); err != nil {
		return fmt.Errorf("initial highlight: %w", err)
	}
	log.Debug("initial highlight ok")

	logFid, err := fs.Open(fmt.Sprintf("%d/log", id), plan9.OREAD)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	log.Debug("watching log")

	timer := time.NewTimer(debounceDuration)
	timer.Stop()
	pending := false

	// lines carries edit notifications (I/D lines) from the scanner goroutine.
	// scanResult carries the exit reason: nil = clean EOF (window closed), else error.
	// goroutineExited is closed after the goroutine writes to scanResult.
	lines := make(chan struct{}, 32)
	scanResult := make(chan error, 1) // buffered so the goroutine never blocks
	goroutineExited := make(chan struct{})

	go func() {
		defer close(goroutineExited)
		sc := bufio.NewScanner(logFid)
		for sc.Scan() {
			line := sc.Text()
			if len(line) >= 1 && (line[0] == 'I' || line[0] == 'D') {
				select {
				case lines <- struct{}{}:
				default:
				}
			}
		}
		scanResult <- sc.Err() // nil = window closed cleanly
	}()

	defer func() {
		logFid.Close()      // unblocks the scanner goroutine
		<-goroutineExited   // wait for it to finish
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
			if err := doHighlight(ctx, id, lang, sl, fs); err != nil {
				return fmt.Errorf("re-highlight: %w", err)
			}
		}
	}
}

// doHighlight reads the window body, parses it with tree-sitter, and writes
// the resulting highlight entries to sl.
func doHighlight(ctx context.Context, id int, lang *Language, sl *layer.StyleLayer, fs *client.Fsys) error {
	log := logger.L(ctx)
	body, err := readBody(id, fs)
	if err != nil {
		return err
	}
	entries := computeHighlights(lang, body)
	log.Debug("highlight entries computed", zap.Int("count", len(entries)))
	return sl.Apply(entries)
}

// readBody reads <id>/body from acme in full.
func readBody(id int, fs *client.Fsys) ([]byte, error) {
	fid, err := fs.Open(fmt.Sprintf("%d/body", id), plan9.OREAD)
	if err != nil {
		return nil, fmt.Errorf("open body: %w", err)
	}
	defer fid.Close()
	return io.ReadAll(fid)
}

// readFirstLine reads the first line of <id>/body for shebang detection.
func readFirstLine(id int, fs *client.Fsys) (string, error) {
	fid, err := fs.Open(fmt.Sprintf("%d/body", id), plan9.OREAD)
	if err != nil {
		return "", fmt.Errorf("open body: %w", err)
	}
	defer fid.Close()
	sc := bufio.NewScanner(fid)
	if sc.Scan() {
		return sc.Text(), nil
	}
	return "", sc.Err()
}
