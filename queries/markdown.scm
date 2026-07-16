; Headings → keyword (bold blue)
(atx_heading (inline) @keyword)
(setext_heading (paragraph) @keyword)

[
  (atx_h1_marker)
  (atx_h2_marker)
  (atx_h3_marker)
  (atx_h4_marker)
  (atx_h5_marker)
  (atx_h6_marker)
  (setext_h1_underline)
  (setext_h2_underline)
] @keyword

; Code blocks → string (red tint)
[
  (indented_code_block)
  (fenced_code_block)
  (link_title)
] @string

(fenced_code_block_delimiter) @operator

; Links
(link_destination) @type
(link_label)      @type

; List markers, block quotes, thematic breaks → operator
[
  (list_marker_plus)
  (list_marker_minus)
  (list_marker_star)
  (list_marker_dot)
  (list_marker_parenthesis)
  (thematic_break)
  (block_continuation)
  (block_quote_marker)
] @operator

; Escape sequences → number (amber)
(backslash_escape) @number
