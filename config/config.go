// Package config handles loading and parsing acme-treesitter's YAML config.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level structure of ~/lib/acme-treesitter/config.yaml.
type Config struct {
	// StyleFile is unused; kept for backward compatibility with existing
	// config files.  The palette is managed by acme-styles.
	StyleFile string `yaml:"style_file"`

	// FilenameHandlers maps filename patterns to grammar language IDs.
	// Evaluated in order; first match wins.  Patterns are Go regular
	// expressions; the same regexes used in acme-lsp's FilenameHandlers work
	// here unchanged.
	FilenameHandlers []FilenameHandler `yaml:"filename_handlers"`

}

// FilenameHandler associates a filename regex pattern with a grammar language ID.
type FilenameHandler struct {
	Pattern    string `yaml:"pattern"`
	LanguageID string `yaml:"language_id"`
}

// Load reads path and returns the parsed Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}
