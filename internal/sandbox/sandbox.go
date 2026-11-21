package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sandbox enforces filesystem access restrictions for PDF operations.
type Sandbox struct {
	allowedDirs  []string
	allowedFiles []string
	cwd          string
	noWorkdir    bool
}

// expandPath expands environment variables and a leading ~ (or ~/ or ~\) using os.ExpandEnv and os.UserHomeDir.
func expandPath(p string) (string, error) {
	p = os.ExpandEnv(p)
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

// New creates a new Sandbox configured with the given root paths.
// The canonical process current working directory (cwd) is always captured.
// If noWorkdir is false, cwd is also added as an allowed directory root.
func New(roots []string, noWorkdir bool) (*Sandbox, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	absWd, err := filepath.Abs(wd)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for working directory: %w", err)
	}
	canonicalWd, err := filepath.EvalSymlinks(absWd)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve working directory symlinks: %w", err)
	}

	s := &Sandbox{
		cwd:       canonicalWd,
		noWorkdir: noWorkdir,
	}

	if !noWorkdir {
		addUnique(&s.allowedDirs, canonicalWd)
	}

	for _, root := range roots {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}

		expanded, err := expandPath(trimmed)
		if err != nil {
			return nil, fmt.Errorf("failed to expand root %q: %w", trimmed, err)
		}

		absRoot, err := filepath.Abs(expanded)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for root %q: %w", trimmed, err)
		}

		canonicalRoot, err := filepath.EvalSymlinks(absRoot)
		if err != nil {
			return nil, fmt.Errorf("root path does not exist or cannot be resolved %q: %w", trimmed, err)
		}

		info, err := os.Stat(canonicalRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to stat root %q: %w", canonicalRoot, err)
		}

		if info.IsDir() {
			addUnique(&s.allowedDirs, canonicalRoot)
		} else if info.Mode().IsRegular() {
			addUnique(&s.allowedFiles, canonicalRoot)
		} else {
			return nil, fmt.Errorf("root path %q is neither a directory nor a regular file", trimmed)
		}
	}

	if noWorkdir && len(s.allowedDirs) == 0 && len(s.allowedFiles) == 0 {
		return nil, errors.New("at least one --root must be specified when --no-workdir is set")
	}

	return s, nil
}

// StatusString returns sandbox debugging details including cwd, workdir_allowed, allowed_roots, and allowed_files.
func (s *Sandbox) StatusString() string {
	return fmt.Sprintf("cwd=%q (workdir_allowed=%t), allowed_roots=%s, allowed_files=%s",
		s.cwd,
		!s.noWorkdir,
		formatQuotedSlice(s.allowedDirs),
		formatQuotedSlice(s.allowedFiles),
	)
}

// ValidatePath checks whether inputPath is allowed by the sandbox, returning the canonical path.
func (s *Sandbox) ValidatePath(inputPath string) (string, error) {
	trimmed := strings.TrimSpace(inputPath)
	if trimmed == "" {
		return "", fmt.Errorf("path is required (%s)", s.StatusString())
	}

	expanded, err := expandPath(trimmed)
	if err != nil {
		return "", fmt.Errorf("failed to expand path %q: %w (%s)", trimmed, err, s.StatusString())
	}

	candidate := expanded
	if !filepath.IsAbs(candidate) {
		if len(s.allowedDirs) > 0 {
			found := false
			for _, dir := range s.allowedDirs {
				target := filepath.Join(dir, candidate)
				if _, err := os.Stat(target); err == nil {
					candidate = target
					found = true
					break
				}
			}
			if !found {
				candidate = filepath.Join(s.allowedDirs[0], candidate)
			}
		} else {
			return "", fmt.Errorf("relative paths require at least one allowed directory root (%s)", s.StatusString())
		}
	}

	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %q: %w (%s)", candidate, err, s.StatusString())
	}

	canonical, err := filepath.EvalSymlinks(absCandidate)
	if err != nil {
		return "", fmt.Errorf("file does not exist or path cannot be resolved %q: %w (%s)", candidate, err, s.StatusString())
	}

	info, err := os.Stat(canonical)
	if err != nil {
		return "", fmt.Errorf("failed to stat path %q: %w (%s)", canonical, err, s.StatusString())
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, expected a PDF file: %q (%s)", inputPath, s.StatusString())
	}

	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("path is not a regular file: %q (%s)", inputPath, s.StatusString())
	}

	for _, file := range s.allowedFiles {
		if canonical == file {
			return canonical, nil
		}
	}

	for _, dir := range s.allowedDirs {
		rel, err := filepath.Rel(dir, canonical)
		if err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return canonical, nil
		}
	}

	return "", fmt.Errorf("access denied: path %q is outside allowed roots (%s)", inputPath, s.StatusString())
}

// AllowedRoots returns the list of allowed root directories and files.
func (s *Sandbox) AllowedRoots() []string {
	roots := make([]string, 0, len(s.allowedDirs)+len(s.allowedFiles))
	roots = append(roots, s.allowedDirs...)
	roots = append(roots, s.allowedFiles...)
	return roots
}

func addUnique(slice *[]string, item string) {
	for _, existing := range *slice {
		if existing == item {
			return
		}
	}
	*slice = append(*slice, item)
}

func formatQuotedSlice(slice []string) string {
	quoted := make([]string, len(slice))
	for i, str := range slice {
		quoted[i] = fmt.Sprintf("%q", str)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
