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
	"flag"
	"log"

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

	if cfg.StyleFile == "" {
		log.Fatal("acme-treesitter: config missing style_file")
	}
	sm, err := LoadStyleFile(cfg.StyleFile)
	if err != nil {
		log.Fatalf("load styles %s: %v", cfg.StyleFile, err)
	}
	if Verbose {
		log.Printf("loaded %d style entries from %s", len(sm), cfg.StyleFile)
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

	// Seed from currently-open windows.
	wins, err := acme.Windows()
	if err != nil {
		log.Fatalf("acme.Windows: %v", err)
	}
	for _, w := range wins {
		go runWindow(w.ID, w.Name, handlers, sm)
	}

	// Stream new opens and closes.
	lr, err := acme.Log()
	if err != nil {
		log.Fatalf("acme.Log: %v", err)
	}
	for {
		ev, err := lr.Read()
		if err != nil {
			log.Fatalf("acme log: %v", err)
		}
		switch ev.Op {
		case "new":
			go runWindow(ev.ID, ev.Name, handlers, sm)
		}
	}
}
