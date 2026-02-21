// acme-treesitter: syntax highlighting for acme via tree-sitter.
//
// Watches acme/log for window opens and closes.  For each window whose
// filename matches a handler in the config, it:
//
//   - allocates a compositor layer in acme-styles,
//   - parses the body with tree-sitter and writes highlight entries, and
//   - re-highlights after any body edit (debounced at 200 ms).
//
// Usage:
//
//	acme-treesitter --config ~/lib/acme-treesitter/config.yaml
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"9fans.net/go/acme"
	ts "github.com/cptaffe/acme-treesitter"
	"github.com/cptaffe/acme-treesitter/config"
	"github.com/cptaffe/acme-treesitter/logger"
	"go.uber.org/zap"
)

func main() {
	cfgPath := flag.String("config", "", "path to config.yaml (required)")
	verbose := flag.Bool("v", false, "verbose logging")
	flag.Parse()

	if *cfgPath == "" {
		log.Fatal("acme-treesitter: --config flag is required")
	}

	var l *zap.Logger
	var err error
	if *verbose {
		l, err = zap.NewDevelopment()
	} else {
		l, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	zap.ReplaceGlobals(l)
	defer l.Sync() //nolint:errcheck

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		l.Fatal("load config", zap.Error(err))
	}

	handlers, err := ts.CompileHandlers(cfg)
	if err != nil {
		l.Fatal("compile filename handlers", zap.Error(err))
	}
	l.Info("handlers compiled", zap.Int("count", len(handlers)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = logger.NewContext(ctx, l)

	// Cancel the context on SIGTERM or SIGINT so per-window goroutines can
	// delete their acme-styles layers before the process exits.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigs
		cancel()
	}()

	var wg sync.WaitGroup

	// active tracks which window IDs currently have a RunWindow goroutine.
	// Guarded by activeMu.  start() is idempotent: safe to call multiple
	// times for the same ID (e.g. on log reconnection after acme restart).
	var activeMu sync.Mutex
	active := make(map[int]struct{})

	start := func(id int, name string) {
		activeMu.Lock()
		if _, ok := active[id]; ok {
			activeMu.Unlock()
			return
		}
		active[id] = struct{}{}
		activeMu.Unlock()

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				activeMu.Lock()
				delete(active, id)
				activeMu.Unlock()
			}()
			ts.RunWindow(ctx, id, name, handlers)
		}()
	}

	// withRetry calls fn until it returns nil or the context is cancelled,
	// sleeping between failures using full-jitter exponential backoff.
	withRetry := func(base, cap time.Duration, fn func() error) error {
		bo := ts.Backoff{Base: base, Cap: cap}
		for {
			if err := fn(); err == nil {
				return nil
			} else if ctx.Err() != nil {
				return ctx.Err()
			} else {
				d := bo.Next()
				l.Warn("retrying", zap.Error(err), zap.Duration("in", d))
				select {
				case <-time.After(d):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}

	// Main reconnect loop.
	//
	// The first call to acme.Windows() and acme.Log() blocks inside the
	// acme package until the background supervisor has established the
	// initial connection to acme (no retry needed here for startup).
	//
	// After a disconnect, fsys goes nil and both calls return "not
	// connected" immediately; withRetry handles the re-connect wait.
	for ctx.Err() == nil {
		// Seed from currently-open windows.
		var wins []acme.WinInfo
		if err := withRetry(200*time.Millisecond, 30*time.Second, func() error {
			var e error
			wins, e = acme.Windows()
			return e
		}); err != nil {
			break
		}
		for _, w := range wins {
			start(w.ID, w.Name)
		}

		// Open the log stream.
		var lr *acme.LogReader
		if err := withRetry(200*time.Millisecond, 30*time.Second, func() error {
			var e error
			lr, e = acme.Log()
			return e
		}); err != nil {
			break
		}

		l.Info("connected to acme log")
		for {
			ev, err := lr.Read()
			if err != nil {
				lr.Close()
				if ctx.Err() != nil {
					goto done
				}
				l.Warn("acme log read error, reconnecting", zap.Error(err))
				break
			}
			switch ev.Op {
			case "new":
				start(ev.ID, ev.Name)
			}
		}
	}

done:
	cancel()
	wg.Wait()
}
