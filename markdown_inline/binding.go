package tree_sitter_markdown_inline

// #cgo CFLAGS: -std=c11 -fPIC
// #include "../vendor/tree-sitter-markdown/tree-sitter-markdown-inline/src/parser.c"
// #include "../vendor/tree-sitter-markdown/tree-sitter-markdown-inline/src/scanner.c"
import "C"

import "unsafe"

// Language returns the tree-sitter Language for Markdown (inline grammar).
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_markdown_inline())
}
