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
)

// Verbose enables extra diagnostic logging.
var Verbose bool

func main() {
	cfgPath := flag.String("config", "", "path to config.yaml (required)")
	flag.BoolVar(&Verbose, "v", false, "verbose logging")
	flag.Parse()

	if *cfgPath == "" {
		log.Fatal("acme-treesitter: --config flag is required")
	}

	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Compile grammars first so handler lookup can resolve language_id strings.
	initLanguages()

	handlers, err := compileHandlers(cfg)
	if err != nil {
		log.Fatalf("compile filename handlers: %v", err)
	}
	if Verbose {
		log.Printf("compiled %d filename handlers", len(handlers))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		go runWindow(ctx, &wg, id, name, handlers)
	}

	// Seed from currently-open windows.
	wins, err := acme.Windows()
	if err != nil {
		log.Fatalf("acme.Windows: %v", err)
	}
	for _, w := range wins {
		start(w.ID, w.Name)
	}

	// Stream new opens and closes.
	lr, err := acme.Log()
	if err != nil {
		log.Fatalf("acme.Log: %v", err)
	}
	for {
		ev, err := lr.Read()
		if err != nil {
			// acme exited or log closed; wait for all windows to clean up.
			cancel()
			wg.Wait()
			log.Fatalf("acme log: %v", err)
		}
		switch ev.Op {
		case "new":
			start(ev.ID, ev.Name)
		}
	}
}
