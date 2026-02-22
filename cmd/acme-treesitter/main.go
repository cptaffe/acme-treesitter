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
	"os/signal"
	"sync"

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


	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer stop()
	ctx = logger.NewContext(ctx, l)

	var wg sync.WaitGroup

	// active tracks which window IDs currently have a RunWindow goroutine.
	// Guarded by activeMu.
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

	f, err := acme.Mount()
	if err != nil {
		l.Fatal("mount acme", zap.Error(err))
	}

	wins, err := f.Windows()
	if err != nil {
		l.Fatal("acme.Windows", zap.Error(err))
	}
	for _, w := range wins {
		start(w.ID, w.Name)
	}

	lr, err := f.Log()
	if err != nil {
		l.Fatal("acme.Log", zap.Error(err))
	}
	defer lr.Close()

	l.Info("connected to acme log")
	for {
		ev, err := lr.Read()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			l.Fatal("acme log read", zap.Error(err))
		}
		switch ev.Op {
		case "new":
			start(ev.ID, ev.Name)
		}
	}

	wg.Wait()
}
