# Release Process

This document describes how to create and publish a new release of `pdf-mcp`.

---

## Prerequisites

1. **Homebrew Tap Repository**: A GitHub repository named `homebrew-tap` under your account
   - Example: `https://github.com/dastrobu/homebrew-tap`
   - Used to publish Homebrew formulae for macOS and Linux users.

2. **GitHub Personal Access Token for Homebrew**:
   - A GitHub Personal Access Token (classic) with `repo` scope.
   - Stored as a secret named `HOMEBREW_TAP_GITHUB_TOKEN` in the repository settings:
     `https://github.com/dastrobu/pdf-mcp/settings/secrets/actions`

---

## Release Steps

### 1. Prepare for Release

Ensure all changes are committed, tested, and pushed to `main`:

```bash
git checkout main
git pull origin main
make check
```

Verify that the CI workflow has passed on GitHub:
https://github.com/dastrobu/pdf-mcp/actions

### 2. Test GoReleaser Locally (Optional Dry-Run)

Test the build configuration locally without publishing:

```bash
goreleaser release --snapshot --clean --skip=publish
```

This will:
- Build binaries for all supported platforms:
  - macOS (`darwin/amd64`, `darwin/arm64`)
  - Linux (`linux/amd64`, `linux/arm64`)
  - Windows (`windows/amd64`, `windows/arm64`)
- Create tar.gz and zip archives
- Generate SHA256 checksums in `dist/`

### 3. Create and Push a Version Tag

Choose a semantic version number (e.g. `v0.1.0`, `v1.0.0`):

```bash
# Create an annotated tag
git tag -a v0.1.0 -m "Release v0.1.0"

# Push the tag to GitHub
git push origin v0.1.0
```

> **Note:** Always use an annotated tag (`-a` flag) rather than a lightweight tag.

### 4. GitHub Actions Automated Pipeline

Pushing the tag triggers `.github/workflows/release.yaml`:

1. Checks out repository with full git history.
2. Runs the Go test suite across all packages.
3. Invokes GoReleaser to cross-compile binaries:
   - macOS (`darwin/amd64`, `darwin/arm64`)
   - Linux (`linux/amd64`, `linux/arm64`)
   - Windows (`windows/amd64`, `windows/arm64`)
4. Generates changelog and creates a GitHub Release.
5. Updates the Homebrew formula in `dastrobu/homebrew-tap`.

### 5. Verify the Release

#### Check GitHub Release

1. Navigate to: https://github.com/dastrobu/pdf-mcp/releases
2. Verify release notes and check all artifacts:
   - `pdf-mcp_v0.1.0_darwin_amd64.tar.gz`
   - `pdf-mcp_v0.1.0_darwin_arm64.tar.gz`
   - `pdf-mcp_v0.1.0_linux_amd64.tar.gz`
   - `pdf-mcp_v0.1.0_linux_arm64.tar.gz`
   - `pdf-mcp_v0.1.0_windows_amd64.zip`
   - `pdf-mcp_v0.1.0_windows_arm64.zip`
   - `pdf-mcp_v0.1.0_checksums.txt`

#### Test Homebrew Installation

```bash
brew tap dastrobu/tap
brew install pdf-mcp
pdf-mcp --version
```

---

## Redoing a Release

If a release needs to be recreated:

```bash
# Delete the local tag
git tag -d v0.1.0

# Delete the remote tag
git push origin :refs/tags/v0.1.0

# Delete the GitHub Release from the web interface

# Recreate and push tag
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

---

## Semantic Versioning

Follow [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR** (`v1.0.0` → `v2.0.0`): Incompatible breaking API or schema changes.
- **MINOR** (`v1.0.0` → `v1.1.0`): New backward-compatible tools or features.
- **PATCH** (`v1.0.0` → `v1.0.1`): Backward-compatible bug fixes and optimizations.
