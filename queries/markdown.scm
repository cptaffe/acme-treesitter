;From nvim-treesitter/nvim-treesitter
(atx_heading
  (inline) @text.title)

(setext_heading
  (paragraph) @text.title)

[
  (atx_h1_marker)
  (atx_h2_marker)
  (atx_h3_marker)
  (atx_h4_marker)
  (atx_h5_marker)
  (atx_h6_marker)
  (setext_h1_underline)
  (setext_h2_underline)
] @punctuation.special

; Note: (fenced_code_block) is intentionally NOT captured here.  The injection
; pass only fills byte positions the primary pass left unclaimed, so claiming
; the whole block as @text.literal would prevent any injected fence language
; (mermaid, python, ...) from highlighting.  The delimiters are still captured
; below as @punctuation.delimiter; the content is left for injection.
[
  (link_title)
  (indented_code_block)
] @text.literal

(fenced_code_block_delimiter) @punctuation.delimiter

(code_fence_content) @none

(link_destination) @text.uri

(link_label) @text.reference

; Tables: header cells read as titles; pipes and the delimiter row are
; punctuation.  (From nvim-treesitter / Helix.)
(pipe_table_header
  (pipe_table_cell) @text.title)

(pipe_table_header
  "|" @punctuation.special)

(pipe_table_row
  "|" @punctuation.special)

(pipe_table_delimiter_row
  "|" @punctuation.special)

(pipe_table_delimiter_cell) @punctuation.special

[
  (list_marker_plus)
  (list_marker_minus)
  (list_marker_star)
  (list_marker_dot)
  (list_marker_parenthesis)
  (thematic_break)
] @punctuation.special

[
  (block_continuation)
  (block_quote_marker)
] @punctuation.special

(backslash_escape) @string.escape
