package tree_sitter_markdown

// #cgo CFLAGS: -std=c11 -fPIC
// extern void *tree_sitter_markdown(void);
import "C"

import "unsafe"

// Language returns the tree-sitter Language for Markdown.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_markdown())
}
