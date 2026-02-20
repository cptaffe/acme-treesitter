package treesitter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"9fans.net/go/plan9"
	"9fans.net/go/plan9/client"
	"github.com/cptaffe/acme-styles/layer"
	"go.uber.org/zap"
)

// Log is the package-level logger.  main should replace it with a configured
// logger (zap.NewDevelopment or zap.NewProduction) before calling RunWindow.
var Log = zap.NewNop()

const layerName = "treesitter"

const debounceDuration = 200 * time.Millisecond

// RunWindow is the per-window goroutine.  It:
//  1. Detects the file's language via the compiled filename handlers.
//  2. Allocates an acme-styles layer for the window.
//  3. Does an initial parse + highlight.
//  4. Reads <winid>/log for body edits; re-highlights after a debounce.
//
// When ctx is cancelled (e.g. on SIGTERM/SIGINT) the layer is deleted from
// the compositor so highlights don't linger after the process exits.
func RunWindow(ctx context.Context, wg interface{ Done() }, id int, name string, handlers []Handler) {
	defer wg.Done()

	log := Log.With(zap.Int("window", id), zap.String("name", name))

	lang := detectLanguage(handlers, name)
	if lang == nil {
		log.Debug("no handler matched")
		return
	}
	log.Debug("matched language", zap.String("lang", lang.Name))

	sl, err := layer.Open(id, layerName)
	if err != nil {
		log.Debug("open layer", zap.Error(err))
	} else {
		log.Debug("allocated layer", zap.Int("layerID", sl.LayerID))
	}
	// sl may be nil if acme-styles is not running; Apply/Clear/Delete are nil-safe.

	// Open a dedicated acme 9P connection for this window's lifetime.
	// Using a fresh connection per goroutine avoids fid-namespace races with
	// the global 9fans.net/go/acme singleton.
	fs, err := client.MountService("acme")
	if err != nil {
		log.Debug("mount acme", zap.Error(err))
		return
	}
	defer fs.Close()

	// Initial highlight.
	if err := doHighlight(id, log, lang, sl, fs); err != nil {
		log.Debug("initial highlight", zap.Error(err))
	} else {
		log.Debug("initial highlight ok")
	}

	// Open <winid>/log for body-edit notifications.
	logFid, err := fs.Open(fmt.Sprintf("%d/log", id), plan9.OREAD)
	if err != nil {
		log.Debug("open log, no incremental re-highlight", zap.Error(err))
		return
	}
	log.Debug("watching log")

	timer := time.NewTimer(debounceDuration)
	timer.Stop()
	pending := false

	// Read log lines in a separate goroutine so the main select can also
	// handle the debounce timer and ctx cancellation.
	lines := make(chan struct{}, 32)
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
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
	}()

	defer func() {
		logFid.Close() // unblocks the scanner goroutine if still running
		<-scanDone     // wait for it to exit before returning
	}()

	for {
		select {
		case <-ctx.Done():
			// Process is exiting: delete the layer so highlights don't linger.
			sl.Delete()
			return

		case _, ok := <-lines:
			if !ok {
				// log EOF — window closed; acme-styles will clean up the WinState.
				sl.Clear()
				return
			}
			if !pending {
				timer.Reset(debounceDuration)
				pending = true
			}

		case <-timer.C:
			pending = false
			if err := doHighlight(id, log, lang, sl, fs); err != nil {
				log.Debug("re-highlight", zap.Error(err))
			}

		case <-scanDone:
			sl.Clear()
			return
		}
	}
}

// doHighlight reads the window body from acme via fs, parses it with
// tree-sitter, and applies the resulting style entries to sl.
func doHighlight(id int, log *zap.Logger, lang *Language, sl *layer.StyleLayer, fs *client.Fsys) error {
	body, err := readBody(id, fs)
	if err != nil {
		return err
	}
	entries := computeHighlights(lang, body)
	log.Debug("highlight entries computed", zap.Int("count", len(entries)))
	if err := sl.Apply(entries); err != nil {
		return err
	}
	log.Debug("Apply ok")
	return nil
}

// readBody reads <id>/body from the given acme 9P connection in full.
func readBody(id int, fs *client.Fsys) ([]byte, error) {
	fid, err := fs.Open(fmt.Sprintf("%d/body", id), plan9.OREAD)
	if err != nil {
		return nil, fmt.Errorf("open body: %w", err)
	}
	defer fid.Close()
	return io.ReadAll(fid)
}
