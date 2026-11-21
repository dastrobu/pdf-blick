# Contributing to pdf-mcp

Thank you for your interest in contributing to `pdf-mcp`! This document provides guidelines and instructions for contributing.

---

## Getting Started

### Prerequisites

- Go 1.27 or later
- Git
- Platform: macOS, Linux, or Windows (cross-platform pure Go via WebAssembly PDFium, no CGO required)

### Setting Up Your Development Environment

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/pdf-mcp.git
   cd pdf-mcp
   ```

3. Add the upstream repository:
   ```bash
   git remote add upstream https://github.com/dastrobu/pdf-mcp.git
   ```

4. Install dependencies:
   ```bash
   go mod download
   ```

5. Install git pre-commit hooks:
   ```bash
   make install-hooks
   ```

6. Build the project:
   ```bash
   make build
   ```

---

## Development Workflow

### Before You Start

1. Check existing issues and pull requests to avoid duplicate work.
2. For major architectural changes or new tools, open an issue first to discuss your proposal.
3. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

### Making Changes

1. **Write clean, idiomatic Go code**
   - Follow standard Go conventions (`gofmt`, `go vet`).
   - Adhere strictly to the critical rules documented in [`AGENTS.md`](AGENTS.md).
   - Format your code: `make fmt`
   - Run static analysis: `make vet`

2. **Test your changes**
   - Write unit tests for any new or modified functionality.
   - Run tests: `make test`
   - Ensure all tests pass before submitting.

3. **Update documentation**
   - Update `README.md` and `AGENTS.md` if you modify CLI options, tool schemas, or engine bounds.
   - Add comments to exported types and functions.

4. **Commit your changes**
   - Write clear, descriptive commit messages following the Conventional Commits format (e.g. `feat:`, `fix:`, `docs:`, `test:`, `refactor:`).

---

## Critical Rules & Guidelines

Always refer to [`AGENTS.md`](AGENTS.md) for full architecture details and design specifications:

### 1. JSON Handling (CRITICAL)
- **Always use `encoding/json/v2`:** Never import or use the legacy `encoding/json` package.
- In Go code, import `"encoding/json/v2"`. For text formatting and indentation options, import `"encoding/json/jsontext"`.

### 2. Struct Tags & JSON Schema (CRITICAL)
- In tool input structs (`ListPagesInput`, `GetPageInput`, etc.), `jsonschema` struct tags **MUST** be plain string descriptions.
- **NEVER** use `key=value` format (such as `jsonschema:"description=...,required"`). `modelcontextprotocol/go-sdk` panics at startup if `key=value` syntax is used in `jsonschema` tags.

### 3. Optional Tool Parameters Must Be Pointers (CRITICAL)
- All optional parameters in tool input structs and engine option structs **MUST** be pointer types (`*int`, `*float64`, `*string`, `*bool`) rather than value types.
- Pointer types allow reliably distinguishing omitted parameters (`nil`) from explicitly passed zero values (`0`, `0.0`, `""`, or `false`).

### 4. No Hardcoded Bounds or Magic Numbers (CRITICAL)
- Operational bounds, rendering limits, pagination windows, and tolerances (e.g. max image dimensions, DPI min/max/default, max pages, bookmark/metadata truncation limits) must **NEVER** be hardcoded inline as magic numbers.
- They must always be defined as named constants in `internal/pdf` and exposed as configurable options through `pdf.Config` and `EngineOpts` (CLI flags + `PDF_MCP_*` environment variables in `internal/opts/opts.go`).

---

## Testing

### Running Tests

```bash
# Run all Go tests
make test

# Run tests with verbose output
go test -v -count=1 ./...

# Run a specific test
go test -v -count=1 ./internal/tools -run TestEndToEndMCPProtocol
```

### Code Quality Verification

Run all checks prior to committing:

```bash
make check
```

This verifies:
- `gofmt` - Code formatting
- `go vet` - Static analysis
- `go test` - Test suite execution

---

## Submitting Your Changes

1. Push your branch to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```

2. Open a pull request against `main` on GitHub:
   - Provide a concise summary of the changes and motivation.
   - Reference any relevant issues.
   - Verify CI passes.

3. Address code review feedback and collaborate with maintainers.

---

## License

By contributing to `pdf-mcp`, you agree that your contributions will be licensed under the MIT License.
