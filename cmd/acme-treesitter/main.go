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

	start := func(id int, name string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ts.RunWindow(ctx, id, name, handlers)
		}()
	}

	// Seed from currently-open windows.  If the index read fails (e.g. acme
	// is mid-update and the line parser chokes on a partial field), just warn
	// and proceed with an empty seed set — new windows arrive via the log.
	wins, err := acme.Windows()
	if err != nil {
		l.Warn("acme.Windows: skipping initial seed", zap.Error(err))
	}
	for _, w := range wins {
		start(w.ID, w.Name)
	}

	// Stream new opens and closes.
	lr, err := acme.Log()
	if err != nil {
		l.Fatal("acme.Log", zap.Error(err))
	}
	for {
		ev, err := lr.Read()
		if err != nil {
			// acme exited or log closed; wait for all windows to clean up.
			cancel()
			wg.Wait()
			l.Fatal("acme log", zap.Error(err))
		}
		switch ev.Op {
		case "new":
			start(ev.ID, ev.Name)
		}
	}
}
