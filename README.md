<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [pdf-blick](#pdf-blick)
  - [Features](#features)
  - [Installation](#installation)
    - [Homebrew (macOS & Linux)](#homebrew-macos--linux)
    - [From Source](#from-source)
  - [Security & Sandbox Model](#security--sandbox-model)
  - [Engine Configuration & Limits](#engine-configuration--limits)
  - [MCP Client Configuration](#mcp-client-configuration)
    - [Claude Desktop](#claude-desktop)
    - [Zed Editor](#zed-editor)
  - [Available MCP Tools](#available-mcp-tools)
    - [`list_pages`](#list_pages)
    - [`get_page`](#get_page)
  - [CLI Usage](#cli-usage)
    - [Server Mode (`run`)](#server-mode-run)
    - [Direct CLI Tool Invocations (`tool`)](#direct-cli-tool-invocations-tool)
    - [Bash Completion (`completion`)](#bash-completion-completion)
  - [Development & Testing](#development--testing)
  - [License](#license)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# pdf-blick

A fast, secure Model Context Protocol (MCP) server for PDF inspection and high-fidelity page rendering over `stdio`.

Built with pure Go and WebAssembly-compiled PDFium (via [wazero](https://github.com/tetratelabs/wazero)), `pdf-blick` runs without CGO, external C library dependencies, or system PDFium installations.

---

## Features

- **Pure Go & Cross-Platform:** Runs PDFium in a WebAssembly sandbox without CGO. Single static binary for macOS, Linux, and Windows (`arm64` & `amd64`).
- **Strict Path Sandboxing:** Restricts PDF file access to explicitly permitted directories and files. Traversal outside allowed roots is blocked. Supports native tilde (`~`) and environment variable expansion.
- **Document Structure Inspection with Context-Window Protection:** Retrieve page counts, outlines/bookmarks, page dimensions, and rotation angles via `list_pages`. Built-in pagination (`start_page`, `end_page`), layout deduplication, and bookmark truncation protect LLM context windows.
- **High-Resolution Page & Region Rendering:** Render entire pages or zoom into normalized bounding boxes (`left`, `top`, `right`, `bottom`) with auto-scaled resolution (up to 600 DPI) via `get_page`.
- **Dual Mode (MCP Server & CLI):** Use as an MCP server with Claude Desktop, Zed, Cursor, or as a standalone CLI tool for scripting.

---

## Installation

### Homebrew (macOS & Linux)

```bash
brew tap dastrobu/tap
brew install pdf-blick
```

### From Source

```bash
git clone https://github.com/dastrobu/pdf-blick.git
cd pdf-blick
make build
```

This generates the `pdf-blick` binary in the current directory.

Or install directly via `go install`:

```bash
go install github.com/dastrobu/pdf-blick@latest
```

---

## Security & Sandbox Model

`pdf-blick` enforces strict path sandboxing to protect host filesystems:

- Paths are resolved to canonical absolute paths (symlinks resolved).
- Access to files outside allowed roots is denied.
- By default, the current working directory (`.`) is allowed.
- `--root` / `-r` (or `PDF_BLICK_ROOT` environment variable, colon-delimited): adds one or more allowed directories or specific file paths. Can be specified multiple times.
- `--no-workdir` (or `PDF_BLICK_NO_WORKDIR=true`): disables implicit access to the current working directory, restricting access strictly to paths passed via `--root`.
- **Native Path Expansion:** Paths passed via `--root`, `PDF_BLICK_ROOT`, or tool input arguments automatically expand leading tildes (`~` / `~/...`) to the user's home directory and expand environment variables (e.g. `$HOME`, `${USER}`).

---

## Engine Configuration & Limits

All operational bounds, rendering limits, and pagination windows can be customized via command-line flags or environment variables:

| Flag | Environment Variable | Default | Description |
|------|----------------------|---------|-------------|
| `--default-dpi` | `PDF_BLICK_DEFAULT_DPI` | `150` | Default rendering resolution in DPI when not specified |
| `--min-dpi` | `PDF_BLICK_MIN_DPI` | `36` | Minimum allowed rendering resolution in DPI |
| `--max-dpi` | `PDF_BLICK_MAX_DPI` | `600` | Maximum allowed rendering resolution in DPI (auto-zoom cap) |
| `--max-image-dimension` | `PDF_BLICK_MAX_IMAGE_DIMENSION` | `4096` | Maximum width or height in pixels for rendered images |
| `--default-max-pages` | `PDF_BLICK_DEFAULT_MAX_PAGES` | `100` | Default max pages returned by `list_pages` when `end_page` omitted |
| `--max-pages-window` | `PDF_BLICK_MAX_PAGES_WINDOW` | `200` | Maximum page window size per `list_pages` call |
| `--max-bookmarks` | `PDF_BLICK_MAX_BOOKMARKS` | `200` | Maximum bookmark/outline nodes returned by `list_pages` |
| `--max-attachments` | `PDF_BLICK_MAX_ATTACHMENTS` | `50` | Maximum attachment names returned by `list_pages` |
| `--max-metadata-len` | `PDF_BLICK_MAX_METADATA_LEN` | `512` | Maximum character length of metadata values before truncation |

---

## MCP Client Configuration

### Claude Desktop

Add `pdf-blick` to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

```json
{
  "mcpServers": {
    "pdf": {
      "command": "/usr/local/bin/pdf-blick",
      "args": [
        "run",
        "--root",
        "/Users/username/Documents"
      ]
    }
  }
}
```

### Zed Editor

Add `pdf-blick` to your Zed settings (`~/.config/zed/settings.json`):

```json
{
  "context_servers": {
    "pdf-blick": {
      "enabled": true,
      "command": "pdf-blick",
      "args": ["run", "--root", "~/Documents/Papers"]
    }
  }
}
```

#### How `pdf-blick` Behaves in Zed (Single-Repo & Multi-Repo Workspaces)

When using `pdf-blick` as a context server inside Zed, keep the following runtime behaviors in mind:

1. **Working Directory (`cwd`) Resolution:**
   When Zed launches a `stdio` context server, it sets the process working directory (`cwd`) to the root of the worktree containing the **currently active editor tab/file**. If no editor tab is active, Zed falls back to the **first visible worktree** in the project panel. Because `pdf-blick` permits access to the current working directory by default, relative PDF paths within that active worktree work immediately out of the box.

2. **Automatic Server Restarts across Worktrees:**
   In multi-repo or multi-root workspaces, whenever your focus moves to an editor tab belonging to a different worktree, Zed detects the working directory change and **automatically restarts** the `stdio` context server with the new worktree root as its `cwd`.

3. **No Shell Expansion in Zed:**
   Zed executes `command` and `args` in `settings.json` directly as a subprocess without passing them through an interactive shell, meaning shell expansions like `~/...` or `$HOME/...` are not expanded by Zed itself. Furthermore, Zed does not currently implement the MCP `roots/list` protocol request. To make setup seamless and ergonomic, `pdf-blick` **natively expands** leading tildes (`~`, `~/...`) and environment variables (`$HOME`, `${VAR}`) across all `--root` arguments, `PDF_BLICK_ROOT` environment variables, and tool `path` inputs.

4. **Multi-Repo & Shared Folder Access:**
   If you work in a multi-repo workspace or need access to PDF files located in external directories (such as reference libraries or document folders), you can configure access in several ways:
   - **Repeatable `--root` flags:**
     ```json
     {
       "context_servers": {
         "pdf-blick": {
           "enabled": true,
           "command": "pdf-blick",
           "args": [
             "run",
             "--root", "~/Documents/Papers",
             "--root", "/Volumes/Shared/Reports"
           ]
         }
       }
     }
     ```
   - **Environment Variable (`PDF_BLICK_ROOT`):**
     ```json
     {
       "context_servers": {
         "pdf-blick": {
           "enabled": true,
           "command": "pdf-blick",
           "args": ["run"],
           "env": {
             "PDF_BLICK_ROOT": "~/Documents/Papers:/Volumes/Shared/Reports"
           }
         }
       }
     }
     ```
   - **Project-Specific Settings:**
     You can also place a `.zed/settings.json` file inside any workspace root to configure project-specific roots or flags that override global user settings.

---

## Available MCP Tools

### `list_pages`

Inspects a PDF document to retrieve total page count, bookmarks/outlines, and per-page dimensions and rotations.

**Parameters:**
- `path` (string, required): Path to the PDF file.
- `start_page` (int, optional): 1-indexed start page for paginating per-page metadata on large PDFs (default: 1).
- `end_page` (int, optional): 1-indexed end page for paginating per-page metadata on large PDFs (default: `start_page + 99`, maximum window: 200 pages).

**Context-Window Protection:**
- **Page Layout Deduplication:** PDFs where all or most pages share identical dimensions and orientations return a consolidated document-level layout definition instead of repeating hundreds of identical page metadata objects.
- **Windowed Pagination:** To protect the LLM context window from being overwhelmed by massive documents, per-page metadata is delivered in windows (defaulting to 100 pages, capped at 200 pages per request).
- **Outline Truncation:** Deep bookmark hierarchies and excessive outlines are safely truncated with structural depth summaries.

**Tool Annotations:**
- `ReadOnlyHint`: `true`
- `IdempotentHint`: `true`
- `DestructiveHint`: `false`
- `OpenWorldHint`: `false`

### `get_page`

Renders a 1-indexed PDF page or normalized crop region (bounding box) to a PNG image.

**Parameters:**
- `path` (string, required): Path to the PDF file.
- `page` (int, required): 1-indexed page number to render.
- `dpi` (int, optional): Resolution in DPI (36-600). Defaults to 150 for full-page, or auto-scaled up to 600 DPI when zooming into a crop box.
- `rotate` (int, optional): Page rotation in degrees clockwise (0, 90, 180, 270, default: 0). Applied before cropping so bounding box coordinates apply to the rotated page.
- `left` (float, optional): Left edge of normalized crop bounding box (0.0 to 1.0, applied after rotation with origin at top-left of the rotated page, default 0.0).
- `top` (float, optional): Top edge of normalized crop bounding box (0.0 to 1.0, applied after rotation with origin at top-left of the rotated page, default 0.0).
- `right` (float, optional): Right edge of normalized crop bounding box (0.0 to 1.0, applied after rotation with origin at top-left of the rotated page, default 1.0).
- `bottom` (float, optional): Bottom edge of normalized crop bounding box (0.0 to 1.0, applied after rotation with origin at top-left of the rotated page, default 1.0).

**Tool Annotations:**
- `ReadOnlyHint`: `true`
- `IdempotentHint`: `true`
- `DestructiveHint`: `false`
- `OpenWorldHint`: `false`

**Output:**
Returns an MCP `ImageContent` object with MIME type `image/png` containing the rendered PNG bytes.

---

## CLI Usage

### Server Mode (`run`)

```bash
# Run over stdio (current working directory allowed)
pdf-blick run

# Run with custom allowed roots and working directory disabled
pdf-blick run --root ~/Documents/Papers -r /Volumes/Shared --no-workdir

# Run with debug logging to stderr
pdf-blick run --debug
```

### Direct CLI Tool Invocations (`tool`)

You can execute tools directly from the terminal without running an MCP server:

```bash
# List pages and outline metadata
pdf-blick tool list_pages --path document.pdf

# List pages with pagination (pages 1 to 50)
pdf-blick tool list_pages --path document.pdf --start-page 1 --end-page 50

# Render a full page and write directly to a PNG file
pdf-blick tool get_page --path document.pdf --page 1 -o page1.png

# Zoom into a specific region (bounding box)
pdf-blick tool get_page --path document.pdf --page 1 --left 0.1 --top 0.2 --right 0.8 --bottom 0.9 -o crop.png

# Render page rotated by 180 degrees (e.g. for upside-down scans)
pdf-blick tool get_page --path document.pdf --page 1 --rotate 180 -o page1_rotated.png

# Render to stdout as JSON
pdf-blick tool get_page --path document.pdf --page 1
```

### Bash Completion (`completion`)

Generate shell completion for bash:

```bash
pdf-blick completion bash > /usr/local/etc/bash_completion.d/pdf-blick
```

---

## Development & Testing

```bash
make build    # Build binary (pdf-blick)
make test     # Run unit tests
make fmt      # Format code with gofmt
make vet      # Run go vet
make check    # Run fmt, vet, test
```

## License

MIT
