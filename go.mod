module github.com/cptaffe/acme-treesitter

go 1.24.4

require (
	9fans.net/go v0.0.7
	github.com/cptaffe/acme-styles v0.0.0-20260220164436-7a3822fafbca
	github.com/tree-sitter-grammars/tree-sitter-toml v0.7.0
	github.com/tree-sitter-grammars/tree-sitter-yaml v0.7.2
	github.com/tree-sitter/go-tree-sitter v0.25.0
	github.com/tree-sitter/tree-sitter-bash v0.25.1
	github.com/tree-sitter/tree-sitter-c v0.24.1
	github.com/tree-sitter/tree-sitter-go v0.25.0
	github.com/tree-sitter/tree-sitter-java v0.23.5
	github.com/tree-sitter/tree-sitter-javascript v0.25.0
	github.com/tree-sitter/tree-sitter-json v0.24.8
	github.com/tree-sitter/tree-sitter-python v0.25.0
	github.com/tree-sitter/tree-sitter-rust v0.24.0
	github.com/tree-sitter/tree-sitter-scala v0.24.0
	github.com/tree-sitter/tree-sitter-typescript v0.23.2
	go.uber.org/zap v1.27.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/mattn/go-pointer v0.0.1 // indirect
	go.uber.org/multierr v1.10.0 // indirect
)

replace 9fans.net/go => ../9fans-go
