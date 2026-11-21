# AGENTS.md

Instructions and guidelines for AI coding agents working on `pdf-mcp`.

## Overview & Architecture

`pdf-mcp` is a Model Context Protocol (MCP) server providing fast, secure, read-only PDF inspection and page rendering over `stdio`. It runs without external C-library dependencies by utilizing WebAssembly-compiled PDFium (via `wazero`), enabling cross-platform, pure-Go operation without CGO.

### Directory Structure

```
pdf-mcp/
├── main.go                      # Application entrypoint & CLI routing
├── Makefile                     # Build & verification recipes
├── .goreleaser.yaml             # Cross-platform release configuration
├── internal/
│   ├── completion/              # Shell auto-completion generators
│   ├── log/                     # Context-aware diagnostic logger
│   ├── opts/                    # CLI parsing (go-flags) & env var handling
│   ├── pdf/                     # WebAssembly PDFium rendering engine
│   ├── sandbox/                 # Path validation & root sandboxing
│   └── tools/                   # MCP tool definitions and handlers
```

## Critical Rules

### 1. JSON Handling (CRITICAL)
- **Always use `encoding/json/v2`:** Never import or use the legacy `encoding/json` package.
- In Go code, import `"encoding/json/v2"`. For text formatting and indentation options, import `"encoding/json/jsontext"`.

### 2. Struct Tags & JSON Schema (CRITICAL)
- In tool input structs (`ListPagesInput`, `GetPageInput`), `jsonschema` struct tags **MUST** be plain string descriptions.
- **NEVER** use `key=value` format (such as `jsonschema:"description=...,required"`). `modelcontextprotocol/go-sdk` panics at startup if `key=value` syntax is used in `jsonschema` tags.

### 3. Optional Tool Parameters Must Be Pointers (CRITICAL)
- All optional parameters in tool input structs (`ListPagesInput`, `GetPageInput`, etc.) and engine option structs **MUST** be pointer types (`*int`, `*float64`, `*string`, `*bool`) rather than value types.
- **Why:** Pointer types allow reliably distinguishing omitted parameters (`nil`) from explicitly passed zero values (such as `0`, `0.0`, `""`, or `false`). This prevents subtle bugs where an explicit `0` (e.g., an invalid 0-indexed page number or 0 DPI) is silently treated as "not set / use default".

### 4. No Hardcoded Bounds or Magic Numbers (CRITICAL)
- Operational bounds, rendering limits, pagination windows, and tolerances (e.g. max image dimensions, DPI min/max/default, max pages, bookmark/metadata truncation limits, paper format tolerances) must **NEVER** be hardcoded inline as magic numbers in functions.
- They must always be defined as named constants in `internal/pdf` and exposed as configurable options through `pdf.Config` and `EngineOpts` (CLI flags + `PDF_MCP_*` environment variables in `internal/opts/opts.go`).

## MCP Tools Specification

Each tool is registered with `modelcontextprotocol/go-sdk/mcp`:

### `list_pages`
- **Purpose:** Inspects PDF document structure, returning total page count, bookmarks/outlines, and per-page dimensions and rotations.
- **Annotations:** `ReadOnlyHint: true`, `IdempotentHint: true`, `DestructiveHint: false`, `OpenWorldHint: false`.
- **Inputs:**
  - `path` (string, required): Path to the PDF file.
  - `start_page` (*int, optional): 1-indexed start page for paginating per-page metadata (defaults to 1).
  - `end_page` (*int, optional): 1-indexed end page for paginating per-page metadata (defaults to `start_page + 99`, max 200 pages per window).
- **Context-Window Protection:**
  - **Layout Deduplication:** Consolidated document-level layout when pages share uniform dimensions and rotations, avoiding repetitive per-page objects.
  - **Windowed Pagination:** Default 100-page window, capped at 200 pages maximum per call.
  - **Bookmark Truncation:** Deep or sprawling outline trees are truncated to protect context limits.
- **Output:** Generic `map[string]any` with metadata.

### `get_page`
- **Purpose:** Renders a 1-indexed PDF page or normalized crop region (bounding box) to a PNG image.
- **Annotations:** `ReadOnlyHint: true`, `IdempotentHint: true`, `DestructiveHint: false`, `OpenWorldHint: false`.
- **Inputs:**
  - `path` (string, required): Path to the PDF file.
  - `page` (int, required): 1-indexed page number to render.
  - `dpi` (*int, optional): Resolution in DPI (36-600). Defaults to 150 for full-page, or auto-scaled up to 600 DPI when zooming into a crop box.
  - `left`, `top`, `right`, `bottom` (*float64, optional): Normalized crop coordinates (0.0 to 1.0, origin at top-left).
- **Output:** `*mcp.CallToolResult` containing `&mcp.ImageContent{ Data: pngBytes, MIMEType: "image/png" }`.

## Security & Sandbox Model

`pdf-mcp` enforces strict path sandboxing:
- All paths are evaluated to canonical absolute paths (resolving symlinks).
- Path traversal outside allowed roots (e.g. `../../etc/passwd`) is denied.
- By default, the current working directory (`.`) is allowed.
- Native path expansion: Leading tilde (`~` / `~/...`) and environment variables (`$HOME`, `${USER}`) are expanded automatically.
- `--root` / `-r` (or `PDF_MCP_ROOT` environment variable, colon-delimited): adds one or more allowed directories or specific file paths.
- `--no-workdir` (or `PDF_MCP_NO_WORKDIR=true`): disables implicit access to the current working directory, restricting access strictly to paths passed via `--root`.

## Engine Configuration & Operational Limits

Operational bounds and rendering limits can be configured via CLI flags or environment variables:

| Flag | Environment Variable | Default | Description |
|------|----------------------|---------|-------------|
| `--default-dpi` | `PDF_MCP_DEFAULT_DPI` | `150` | Default rendering resolution in DPI when not specified |
| `--min-dpi` | `PDF_MCP_MIN_DPI` | `36` | Minimum allowed rendering resolution in DPI |
| `--max-dpi` | `PDF_MCP_MAX_DPI` | `600` | Maximum allowed rendering resolution in DPI (auto-zoom cap) |
| `--max-image-dimension` | `PDF_MCP_MAX_IMAGE_DIMENSION` | `4096` | Maximum width or height in pixels for rendered images |
| `--default-max-pages` | `PDF_MCP_DEFAULT_MAX_PAGES` | `100` | Default max pages returned by `list_pages` when `end_page` omitted |
| `--max-pages-window` | `PDF_MCP_MAX_PAGES_WINDOW` | `200` | Maximum page window size per `list_pages` call |
| `--max-bookmarks` | `PDF_MCP_MAX_BOOKMARKS` | `200` | Maximum bookmark/outline nodes returned by `list_pages` |
| `--max-attachments` | `PDF_MCP_MAX_ATTACHMENTS` | `50` | Maximum attachment names returned by `list_pages` |
| `--max-metadata-len` | `PDF_MCP_MAX_METADATA_LEN` | `512` | Maximum character length of metadata values before truncation |

## CLI Usage

### Server Mode
```bash
# Run over stdio (default allowed root: current directory)
pdf-mcp run

# Run with custom allowed roots and working directory disabled
pdf-mcp run --root ~/Documents/Papers -r /Volumes/Shared --no-workdir

# Run with debug logging to stderr
pdf-mcp run --debug
```

### Direct CLI Tool Invocations
```bash
# List pages and metadata
pdf-mcp tool list_pages --path doc.pdf

# List pages with pagination window
pdf-mcp tool list_pages --path doc.pdf --start-page 1 --end-page 50

# Render full page to stdout JSON or save directly to a file
pdf-mcp tool get_page --path doc.pdf --page 1 -o page1.png

# Render cropped bounding box (zoom into region)
pdf-mcp tool get_page --path doc.pdf --page 1 --left 0.1 --top 0.2 --right 0.8 --bottom 0.9 -o crop.png
```

### Bash Completion
```bash
pdf-mcp completion bash > /usr/local/etc/bash_completion.d/pdf-mcp
```

## Build & Testing

```bash
make build    # Build binary (pdf-mcp)
make test     # Run unit tests
make fmt      # Format code with gofmt
make vet      # Run go vet
make check    # Run fmt, vet, test
```
