# 00017. Native SVG Diagram and Markdown Rendering Without Browser Engine

## Context
The GUI monitors need to render agent messages containing rich GitHub-Flavored Markdown (GFM), syntax-highlighted code blocks, and Mermaid diagrams. Embedding a full browser engine (such as WebKitGTK or QtWebEngine/Chromium) introduces 150MB+ binary bloat, hundreds of megabytes of RAM consumption per process, and complex multi-process sandboxing issues.

## Decision
Render Markdown, code syntax, and diagrams natively without an embedded browser engine:
1. **Native Markdown AST**: Parse Markdown using a fast C/Rust/Go parser (`cmark-gfm`), mapping AST nodes directly into native GTK widget nodes (`GtkLabel` with Pango markup, `GtkGrid` for tables).
2. **Code Syntax Highlighting**: Render code fences using `GtkSourceView-5`, providing native, theme-aware syntax highlighting with zero web rendering overhead.
3. **Mermaid via Background SVG Worker**:
   - Extract ` ```mermaid ` code blocks from markdown text.
   - A background worker converts Mermaid text to pure SVG XML (cached locally by SHA-256 hash).
   - Render the resulting SVG as a native vector graphic in GTK using **`librsvg`** and `GtkPicture` (`gtk_picture_new_for_paintable`).

## Status
Accepted.

## Consequences
- High-fidelity visual rendering of text, syntax-highlighted code, and crisp vector diagrams.
- Retains a lightweight memory footprint (<50 MB) and instant window startup.
- Eliminates heavy browser engine dependencies (WebKit / Chromium / Ghostscript).
