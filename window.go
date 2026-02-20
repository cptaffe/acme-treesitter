package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"time"

	"9fans.net/go/plan9"
	"9fans.net/go/plan9/client"
)

const debounceDuration = 200 * time.Millisecond

// runWindow is the per-window goroutine.  It:
//  1. Detects the file's language via the compiled filename handlers.
//  2. Allocates an acme-styles layer for the window.
//  3. Does an initial parse + highlight.
//  4. Reads <winid>/log for body edits; re-highlights after a debounce.
func runWindow(id int, name string, handlers []runtimeHandler, sm StyleMap) {
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

	layer := newStyleLayer(id)
	if Verbose {
		if layer == nil {
			log.Printf("window %d: newStyleLayer returned nil (acme-styles not running?)", id)
		} else {
			log.Printf("window %d: allocated layer %d", id, layer.layerID)
		}
	}
	// layer may be nil if acme-styles is not running; Apply/Clear are nil-safe.

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
	if err := highlight(id, lang, layer, sm, fs); err != nil {
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
	defer logFid.Close()

	timer := time.NewTimer(debounceDuration)
	timer.Stop()
	pending := false

	// Read log lines in a separate goroutine so the main select can also
	// handle the debounce timer.
	lines := make(chan struct{}, 32)
	done := make(chan struct{})
	go func() {
		defer close(done)
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

	for {
		select {
		case _, ok := <-lines:
			if !ok {
				// log EOF — window closed before done fires
				layer.Clear()
				return
			}
			if !pending {
				timer.Reset(debounceDuration)
				pending = true
			}
		case <-timer.C:
			pending = false
			if err := highlight(id, lang, layer, sm, fs); err != nil && Verbose {
				log.Printf("window %d: re-highlight: %v", id, err)
			}
		case <-done:
			layer.Clear()
			return
		}
	}
}

// highlight reads the window body from acme via fs, parses it with
// tree-sitter, and applies the resulting style entries to layer.
func highlight(id int, lang *Language, layer *StyleLayer, sm StyleMap, fs *client.Fsys) error {
	body, err := readBody(id, fs)
	if err != nil {
		return err
	}
	entries := computeHighlights(lang, body, sm)
	if Verbose {
		log.Printf("window %d: %d highlight entries computed", id, len(entries))
	}
	if err := layer.Apply(entries); err != nil {
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
