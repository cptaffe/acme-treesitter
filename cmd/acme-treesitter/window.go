package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"9fans.net/go/plan9"
	"9fans.net/go/plan9/client"
	"github.com/cptaffe/acme-styles/layer"
)

const layerName = "treesitter"

const debounceDuration = 200 * time.Millisecond

// runWindow is the per-window goroutine.  It:
//  1. Detects the file's language via the compiled filename handlers.
//  2. Allocates an acme-styles layer for the window.
//  3. Does an initial parse + highlight.
//  4. Reads <winid>/log for body edits; re-highlights after a debounce.
//
// When ctx is cancelled (e.g. on SIGTERM/SIGINT) the layer is deleted from
// the compositor so highlights don't linger after the process exits.
func runWindow(ctx context.Context, wg interface{ Done() }, id int, name string, handlers []runtimeHandler) {
	defer wg.Done()

	lang := detectLanguageForName(handlers, name)
	if lang == nil {
		if Verbose {
			log.Printf("window %d %q: no handler matched", id, name)
		}
		return
	}
	if Verbose {
		log.Printf("window %d %q: matched language %s", id, name, lang.Name)
	}

	sl, err := layer.Open(id, layerName)
	if err != nil && Verbose {
		log.Printf("window %d: open layer: %v (acme-styles not running?)", id, err)
	}
	if Verbose && sl != nil {
		log.Printf("window %d: allocated layer %d", id, sl.LayerID)
	}
	// sl may be nil if acme-styles is not running; Apply/Clear/Delete are nil-safe.

	// Open a dedicated acme 9P connection for this window's lifetime.
	// Using a fresh connection per goroutine avoids fid-namespace races with
	// the global 9fans.net/go/acme singleton.
	fs, err := client.MountService("acme")
	if err != nil {
		if Verbose {
			log.Printf("window %d: mount acme: %v", id, err)
		}
		return
	}
	defer fs.Close()

	// Initial highlight.
	if err := highlight(id, lang, sl, fs); err != nil {
		if Verbose {
			log.Printf("window %d %s: initial highlight: %v", id, name, err)
		}
	} else if Verbose {
		log.Printf("window %d %s: initial highlight ok", id, name)
	}

	// Open <winid>/log for body-edit notifications.
	logFid, err := fs.Open(fmt.Sprintf("%d/log", id), plan9.OREAD)
	if err != nil {
		if Verbose {
			log.Printf("window %d: open log: %v (no incremental re-highlight)", id, err)
		}
		return
	}
	if Verbose {
		log.Printf("window %d: watching log", id)
	}

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
		logFid.Close()  // unblocks the scanner goroutine if still running
		<-scanDone      // wait for it to exit before returning
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
			if err := highlight(id, lang, sl, fs); err != nil && Verbose {
				log.Printf("window %d: re-highlight: %v", id, err)
			}

		case <-scanDone:
			sl.Clear()
			return
		}
	}
}

// highlight reads the window body from acme via fs, parses it with
// tree-sitter, and applies the resulting style entries to sl.
func highlight(id int, lang *Language, sl *layer.StyleLayer, fs *client.Fsys) error {
	body, err := readBody(id, fs)
	if err != nil {
		return err
	}
	entries := computeHighlights(lang, body)
	if Verbose {
		log.Printf("window %d: %d highlight entries computed", id, len(entries))
	}
	if err := sl.Apply(entries); err != nil {
		return err
	}
	if Verbose {
		log.Printf("window %d: Apply ok", id)
	}
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
