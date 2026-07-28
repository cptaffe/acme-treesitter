package tree_sitter_mermaid

// #cgo CFLAGS: -std=c11 -fPIC
// #include "../vendor/tree-sitter-mermaid/src/parser.c"
import "C"

import "unsafe"

// Language returns the tree-sitter Language for Mermaid.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_mermaid())
}
