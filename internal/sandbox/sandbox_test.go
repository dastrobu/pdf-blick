package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks on tempdir: %v", err)
	}

	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	testFile := filepath.Join(tempDir, "test.pdf")
	if err := os.WriteFile(testFile, []byte("%PDF-1.4 test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		roots       []string
		noWorkdir   bool
		wantErr     bool
		errContains string
		checkRoots  func(t *testing.T, s *Sandbox)
	}{
		{
			name:      "default workdir enabled with no extra roots",
			roots:     nil,
			noWorkdir: false,
			wantErr:   false,
			checkRoots: func(t *testing.T, s *Sandbox) {
				wd, err := os.Getwd()
				if err != nil {
					t.Fatalf("failed to getwd: %v", err)
				}
				canonWd, err := filepath.EvalSymlinks(wd)
				if err != nil {
					t.Fatalf("failed to eval symlinks on wd: %v", err)
				}
				roots := s.AllowedRoots()
				if len(roots) != 1 || roots[0] != canonWd {
					t.Errorf("expected allowed roots to contain workdir %q, got %v", canonWd, roots)
				}
				if !strings.Contains(s.StatusString(), "workdir_allowed=true") {
					t.Errorf("expected workdir_allowed=true in status: %s", s.StatusString())
				}
			},
		},
		{
			name:        "no-workdir with empty roots list",
			roots:       []string{},
			noWorkdir:   true,
			wantErr:     true,
			errContains: "at least one --root must be specified when --no-workdir is set",
		},
		{
			name:        "no-workdir with whitespace-only roots",
			roots:       []string{"", "   "},
			noWorkdir:   true,
			wantErr:     true,
			errContains: "at least one --root must be specified when --no-workdir is set",
		},
		{
			name:        "non-existent root",
			roots:       []string{filepath.Join(tempDir, "does-not-exist")},
			noWorkdir:   false,
			wantErr:     true,
			errContains: "root path does not exist",
		},
		{
			name:      "no-workdir with valid directory root captures cwd",
			roots:     []string{subDir},
			noWorkdir: true,
			wantErr:   false,
			checkRoots: func(t *testing.T, s *Sandbox) {
				roots := s.AllowedRoots()
				if len(roots) != 1 || roots[0] != subDir {
					t.Errorf("expected allowed roots [%q], got %v", subDir, roots)
				}
				status := s.StatusString()
				if !strings.Contains(status, "cwd=") || !strings.Contains(status, "workdir_allowed=false") {
					t.Errorf("expected cwd and workdir_allowed=false in status: %s", status)
				}
			},
		},
		{
			name:      "no-workdir with valid file root",
			roots:     []string{testFile},
			noWorkdir: true,
			wantErr:   false,
			checkRoots: func(t *testing.T, s *Sandbox) {
				roots := s.AllowedRoots()
				if len(roots) != 1 || roots[0] != testFile {
					t.Errorf("expected allowed roots [%q], got %v", testFile, roots)
				}
				status := s.StatusString()
				if !strings.Contains(status, "allowed_files=[\""+testFile+"\"]") {
					t.Errorf("expected allowed_files in status: %s", status)
				}
			},
		},
		{
			name:      "deduplicates repeated roots",
			roots:     []string{subDir, subDir, testFile, testFile},
			noWorkdir: true,
			wantErr:   false,
			checkRoots: func(t *testing.T, s *Sandbox) {
				roots := s.AllowedRoots()
				if len(roots) != 2 {
					t.Errorf("expected 2 unique roots, got %d (%v)", len(roots), roots)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(tt.roots, tt.noWorkdir)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain expected substring %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.checkRoots != nil {
				tt.checkRoots(t, s)
			}
		})
	}
}

func TestNew_Expansion(t *testing.T) {
	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks on tempdir: %v", err)
	}

	envDir := filepath.Join(tempDir, "from_env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir envDir: %v", err)
	}
	t.Setenv("TEST_EXPAND_ROOT", envDir)

	homeDir := filepath.Join(tempDir, "mock_home")
	mockSubDir := filepath.Join(homeDir, "papers")
	if err := os.MkdirAll(mockSubDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir mockSubDir: %v", err)
	}
	t.Setenv("HOME", homeDir)

	s, err := New([]string{"$TEST_EXPAND_ROOT", "~/papers"}, true)
	if err != nil {
		t.Fatalf("expected New with expanded roots to succeed, got: %v", err)
	}

	allowed := s.AllowedRoots()
	if len(allowed) != 2 {
		t.Fatalf("expected 2 allowed roots, got %d: %v", len(allowed), allowed)
	}
	if allowed[0] != envDir {
		t.Errorf("expected root 0 to be %q, got %q", envDir, allowed[0])
	}
	if allowed[1] != mockSubDir {
		t.Errorf("expected root 1 to be %q, got %q", mockSubDir, allowed[1])
	}
}

func TestExpandPath(t *testing.T) {
	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks on tempdir: %v", err)
	}
	t.Setenv("HOME", tempDir)
	t.Setenv("MY_DIR", "sub/folder")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde alone",
			input:    "~",
			expected: tempDir,
		},
		{
			name:     "tilde slash",
			input:    "~/documents/file.pdf",
			expected: filepath.Join(tempDir, "documents/file.pdf"),
		},
		{
			name:     "env var expansion",
			input:    "$MY_DIR/file.pdf",
			expected: "sub/folder/file.pdf",
		},
		{
			name:     "combined tilde and env var",
			input:    "~/$MY_DIR/file.pdf",
			expected: filepath.Join(tempDir, "sub/folder/file.pdf"),
		},
		{
			name:     "plain relative path",
			input:    "simple/path.pdf",
			expected: "simple/path.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandPath(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestStatusString(t *testing.T) {
	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks on tempdir: %v", err)
	}

	dir1 := filepath.Join(tempDir, "dir1")
	if err := os.MkdirAll(dir1, 0o755); err != nil {
		t.Fatalf("failed to mkdir dir1: %v", err)
	}
	file1 := filepath.Join(tempDir, "file1.pdf")
	if err := os.WriteFile(file1, []byte("%PDF-1.4 file1"), 0o644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}

	s, err := New([]string{dir1, file1}, false)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	status := s.StatusString()
	if !strings.HasPrefix(status, "cwd=\"") {
		t.Errorf("expected status to start with cwd=\", got %s", status)
	}
	if !strings.Contains(status, "(workdir_allowed=true)") {
		t.Errorf("expected workdir_allowed=true in status, got %s", status)
	}
	if !strings.Contains(status, "allowed_roots=") {
		t.Errorf("expected allowed_roots= in status, got %s", status)
	}
	if !strings.Contains(status, "allowed_files=[\""+file1+"\"]") {
		t.Errorf("expected allowed_files to contain file1, got %s", status)
	}
}

func TestValidatePath_DefaultWorkdir(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()

	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks on tempdir: %v", err)
	}

	workDir := filepath.Join(tempDir, "workdir")
	outsideDir := filepath.Join(tempDir, "outside")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir workDir: %v", err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir outsideDir: %v", err)
	}

	childFile := filepath.Join(workDir, "doc.pdf")
	if err := os.WriteFile(childFile, []byte("%PDF-1.4 child"), 0o644); err != nil {
		t.Fatalf("failed to write childFile: %v", err)
	}

	subDir := filepath.Join(workDir, "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir subDir: %v", err)
	}
	nestedFile := filepath.Join(subDir, "nested.pdf")
	if err := os.WriteFile(nestedFile, []byte("%PDF-1.4 nested"), 0o644); err != nil {
		t.Fatalf("failed to write nestedFile: %v", err)
	}

	outsideFile := filepath.Join(outsideDir, "secret.pdf")
	if err := os.WriteFile(outsideFile, []byte("%PDF-1.4 secret"), 0o644); err != nil {
		t.Fatalf("failed to write outsideFile: %v", err)
	}

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("failed to chdir to workDir: %v", err)
	}

	s, err := New(nil, false)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		wantPath    string
		wantErr     bool
		errContains []string
	}{
		{
			name:        "empty path",
			input:       "",
			wantErr:     true,
			errContains: []string{"path is required", "cwd=", "allowed_roots=", "allowed_files="},
		},
		{
			name:        "whitespace only path",
			input:       "   \t\n ",
			wantErr:     true,
			errContains: []string{"path is required", "cwd=", "allowed_roots=", "allowed_files="},
		},
		{
			name:     "child file relative path",
			input:    "doc.pdf",
			wantPath: childFile,
			wantErr:  false,
		},
		{
			name:     "nested child file relative path",
			input:    "nested/nested.pdf",
			wantPath: nestedFile,
			wantErr:  false,
		},
		{
			name:     "child file absolute path",
			input:    childFile,
			wantPath: childFile,
			wantErr:  false,
		},
		{
			name:        "parent relative escape rejected",
			input:       "../outside/secret.pdf",
			wantErr:     true,
			errContains: []string{"access denied", "cwd=", "allowed_roots=", "allowed_files="},
		},
		{
			name:        "outside file absolute path rejected",
			input:       outsideFile,
			wantErr:     true,
			errContains: []string{"access denied", "cwd=", "allowed_roots=", "allowed_files="},
		},
		{
			name:        "non-existent file",
			input:       "notfound.pdf",
			wantErr:     true,
			errContains: []string{"cannot be resolved", filepath.Join(workDir, "notfound.pdf"), "cwd=", "allowed_roots=", "allowed_files="},
		},
		{
			name:        "directory path rejected",
			input:       "nested",
			wantErr:     true,
			errContains: []string{"path is a directory, expected a PDF file", "cwd=", "allowed_roots=", "allowed_files="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.ValidatePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got path: %s", got)
				}
				for _, sub := range tt.errContains {
					if !strings.Contains(err.Error(), sub) {
						t.Errorf("error %q does not contain expected substring %q", err.Error(), sub)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantPath {
				t.Errorf("expected path %q, got %q", tt.wantPath, got)
			}
		})
	}
}

func TestValidatePath_Expansion(t *testing.T) {
	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks on tempdir: %v", err)
	}

	mockHome := filepath.Join(tempDir, "user_home")
	docsDir := filepath.Join(mockHome, "Documents", "Papers")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir docsDir: %v", err)
	}
	t.Setenv("HOME", mockHome)

	docFile := filepath.Join(docsDir, "paper.pdf")
	if err := os.WriteFile(docFile, []byte("%PDF-1.4 paper"), 0o644); err != nil {
		t.Fatalf("failed to write docFile: %v", err)
	}
	t.Setenv("MY_PAPERS_DIR", docsDir)

	s, err := New([]string{"~/Documents"}, true)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	t.Run("tilde expansion in ValidatePath", func(t *testing.T) {
		got, err := s.ValidatePath("~/Documents/Papers/paper.pdf")
		if err != nil {
			t.Fatalf("expected ValidatePath with tilde to succeed, got: %v", err)
		}
		if got != docFile {
			t.Errorf("expected %q, got %q", docFile, got)
		}
	})

	t.Run("env var expansion in ValidatePath", func(t *testing.T) {
		got, err := s.ValidatePath("$MY_PAPERS_DIR/paper.pdf")
		if err != nil {
			t.Fatalf("expected ValidatePath with env var to succeed, got: %v", err)
		}
		if got != docFile {
			t.Errorf("expected %q, got %q", docFile, got)
		}
	})
}

func TestValidatePath_RelativeResolvesToAdditionalRoot(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()

	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks on tempdir: %v", err)
	}

	workDir := filepath.Join(tempDir, "wd")
	extraDir := filepath.Join(tempDir, "extra")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir workDir: %v", err)
	}
	if err := os.MkdirAll(extraDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir extraDir: %v", err)
	}

	// File only exists in extraDir, NOT in workDir
	extraFile := filepath.Join(extraDir, "doc_in_extra.pdf")
	if err := os.WriteFile(extraFile, []byte("%PDF-1.4 extra"), 0o644); err != nil {
		t.Fatalf("failed to write extraFile: %v", err)
	}

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("failed to chdir to workDir: %v", err)
	}

	s, err := New([]string{extraDir}, false)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Relative path should resolve against extraDir because it does not exist in workDir
	got, err := s.ValidatePath("doc_in_extra.pdf")
	if err != nil {
		t.Fatalf("expected relative path to resolve against extraDir, got error: %v", err)
	}
	if got != extraFile {
		t.Errorf("expected resolved path %q, got %q", extraFile, got)
	}
}

func TestValidatePath_AdditionalRoots(t *testing.T) {
	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks on tempdir: %v", err)
	}

	extraDir := filepath.Join(tempDir, "extra_dir")
	if err := os.MkdirAll(extraDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir extraDir: %v", err)
	}
	extraDirFile := filepath.Join(extraDir, "doc.pdf")
	if err := os.WriteFile(extraDirFile, []byte("%PDF-1.4 extra"), 0o644); err != nil {
		t.Fatalf("failed to write extraDirFile: %v", err)
	}

	singleFileDir := filepath.Join(tempDir, "single_file_dir")
	if err := os.MkdirAll(singleFileDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir singleFileDir: %v", err)
	}
	singleFile := filepath.Join(singleFileDir, "allowed.pdf")
	if err := os.WriteFile(singleFile, []byte("%PDF-1.4 allowed"), 0o644); err != nil {
		t.Fatalf("failed to write singleFile: %v", err)
	}
	disallowedFile := filepath.Join(singleFileDir, "disallowed.pdf")
	if err := os.WriteFile(disallowedFile, []byte("%PDF-1.4 disallowed"), 0o644); err != nil {
		t.Fatalf("failed to write disallowedFile: %v", err)
	}

	s, err := New([]string{extraDir, singleFile}, false)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		wantPath    string
		wantErr     bool
		errContains []string
	}{
		{
			name:     "file inside extra root directory",
			input:    extraDirFile,
			wantPath: extraDirFile,
			wantErr:  false,
		},
		{
			name:     "explicit single file root",
			input:    singleFile,
			wantPath: singleFile,
			wantErr:  false,
		},
		{
			name:        "sibling of single file root is denied",
			input:       disallowedFile,
			wantErr:     true,
			errContains: []string{"access denied", "cwd=", "allowed_roots=", "allowed_files="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.ValidatePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got path: %s", got)
				}
				for _, sub := range tt.errContains {
					if !strings.Contains(err.Error(), sub) {
						t.Errorf("error %q does not contain expected substring %q", err.Error(), sub)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantPath {
				t.Errorf("expected path %q, got %q", tt.wantPath, got)
			}
		})
	}
}

func TestValidatePath_NoWorkdir(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()

	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks on tempdir: %v", err)
	}

	workDir := filepath.Join(tempDir, "current_wd")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir workDir: %v", err)
	}
	wdFile := filepath.Join(workDir, "workdir_file.pdf")
	if err := os.WriteFile(wdFile, []byte("%PDF-1.4 wd"), 0o644); err != nil {
		t.Fatalf("failed to write wdFile: %v", err)
	}

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("failed to chdir to workDir: %v", err)
	}

	root1 := filepath.Join(tempDir, "root1")
	root2 := filepath.Join(tempDir, "root2")
	if err := os.MkdirAll(root1, 0o755); err != nil {
		t.Fatalf("failed to mkdir root1: %v", err)
	}
	if err := os.MkdirAll(root2, 0o755); err != nil {
		t.Fatalf("failed to mkdir root2: %v", err)
	}

	file1 := filepath.Join(root1, "file1.pdf")
	file2 := filepath.Join(root2, "file2.pdf")
	if err := os.WriteFile(file1, []byte("%PDF-1.4 file1"), 0o644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("%PDF-1.4 file2"), 0o644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	singleFileRoot := filepath.Join(tempDir, "single.pdf")
	if err := os.WriteFile(singleFileRoot, []byte("%PDF-1.4 single"), 0o644); err != nil {
		t.Fatalf("failed to write singleFileRoot: %v", err)
	}

	t.Run("single root directory allows relative paths resolved to root", func(t *testing.T) {
		s, err := New([]string{root1}, true)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}

		// Relative path resolves to root1
		got, err := s.ValidatePath("file1.pdf")
		if err != nil {
			t.Fatalf("expected success with relative path, got: %v", err)
		}
		if got != file1 {
			t.Errorf("expected %q, got %q", file1, got)
		}

		// Absolute path inside root1 allowed
		got, err = s.ValidatePath(file1)
		if err != nil {
			t.Fatalf("expected success with absolute path, got: %v", err)
		}
		if got != file1 {
			t.Errorf("expected %q, got %q", file1, got)
		}

		// File in current working directory rejected
		_, err = s.ValidatePath(wdFile)
		if err == nil {
			t.Fatalf("expected access denied for working directory file")
		}
		if !strings.Contains(err.Error(), "access denied") {
			t.Errorf("expected 'access denied' error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "cwd=") || !strings.Contains(err.Error(), "allowed_roots=") {
			t.Errorf("expected status info in error: %v", err)
		}
	})

	t.Run("single root file without directory roots rejects relative paths but allows exact file", func(t *testing.T) {
		s, err := New([]string{singleFileRoot}, true)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}

		// Relative path rejected with status info
		_, err = s.ValidatePath("single.pdf")
		if err == nil {
			t.Fatalf("expected error for relative path with single file root")
		}
		if !strings.Contains(err.Error(), "relative paths require at least one allowed directory root") {
			t.Errorf("unexpected error message: %v", err)
		}
		if !strings.Contains(err.Error(), "cwd=") || !strings.Contains(err.Error(), "allowed_files=") {
			t.Errorf("expected status info in error: %v", err)
		}

		// Absolute path to the file allowed
		got, err := s.ValidatePath(singleFileRoot)
		if err != nil {
			t.Fatalf("expected success for single root file, got: %v", err)
		}
		if got != singleFileRoot {
			t.Errorf("expected %q, got %q", singleFileRoot, got)
		}
	})

	t.Run("multiple root directories search across roots for relative paths", func(t *testing.T) {
		s, err := New([]string{root1, root2}, true)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}

		// Relative path resolves file1 from root1
		got1, err := s.ValidatePath("file1.pdf")
		if err != nil || got1 != file1 {
			t.Errorf("expected %q, got %q (err: %v)", file1, got1, err)
		}

		// Relative path resolves file2 from root2
		got2, err := s.ValidatePath("file2.pdf")
		if err != nil || got2 != file2 {
			t.Errorf("expected %q, got %q (err: %v)", file2, got2, err)
		}

		// Non-existent relative file defaults to root1 and returns error with status
		_, err = s.ValidatePath("nonexistent.pdf")
		if err == nil {
			t.Fatalf("expected error for nonexistent relative file")
		}
		if !strings.Contains(err.Error(), filepath.Join(root1, "nonexistent.pdf")) {
			t.Errorf("expected attempted path %q in error: %v", filepath.Join(root1, "nonexistent.pdf"), err)
		}
		if !strings.Contains(err.Error(), "cwd=") || !strings.Contains(err.Error(), "allowed_roots=") {
			t.Errorf("expected status info in error: %v", err)
		}

		// File outside both roots denied
		_, err = s.ValidatePath(wdFile)
		if err == nil || !strings.Contains(err.Error(), "access denied") {
			t.Errorf("expected access denied error, got: %v", err)
		}
	})
}

func TestValidatePath_SymlinkEscape(t *testing.T) {
	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks on tempdir: %v", err)
	}

	allowedDir := filepath.Join(tempDir, "allowed")
	outsideDir := filepath.Join(tempDir, "outside")
	if err := os.MkdirAll(allowedDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir allowedDir: %v", err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir outsideDir: %v", err)
	}

	outsideTarget := filepath.Join(outsideDir, "target.pdf")
	if err := os.WriteFile(outsideTarget, []byte("%PDF-1.4 secret outside"), 0o644); err != nil {
		t.Fatalf("failed to write outsideTarget: %v", err)
	}

	insideTarget := filepath.Join(allowedDir, "real.pdf")
	if err := os.WriteFile(insideTarget, []byte("%PDF-1.4 inside"), 0o644); err != nil {
		t.Fatalf("failed to write insideTarget: %v", err)
	}

	// Symlink pointing outside allowed roots
	escapeLink := filepath.Join(allowedDir, "escape_link.pdf")
	if err := os.Symlink(outsideTarget, escapeLink); err != nil {
		t.Fatalf("failed to create escape symlink: %v", err)
	}

	// Symlink pointing to a file inside allowed roots
	validLink := filepath.Join(allowedDir, "valid_link.pdf")
	if err := os.Symlink(insideTarget, validLink); err != nil {
		t.Fatalf("failed to create valid symlink: %v", err)
	}

	s, err := New([]string{allowedDir}, true)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	t.Run("symlink escaping allowed root is rejected", func(t *testing.T) {
		_, err := s.ValidatePath(escapeLink)
		if err == nil {
			t.Fatalf("expected symlink escape to be rejected, but it succeeded")
		}
		if !strings.Contains(err.Error(), "access denied") {
			t.Errorf("expected access denied error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "cwd=") || !strings.Contains(err.Error(), "allowed_roots=") {
			t.Errorf("expected status info in error: %v", err)
		}
	})

	t.Run("symlink within allowed root is allowed", func(t *testing.T) {
		got, err := s.ValidatePath(validLink)
		if err != nil {
			t.Fatalf("expected symlink within allowed root to succeed, got: %v", err)
		}
		if got != insideTarget {
			t.Errorf("expected canonical target %q, got %q", insideTarget, got)
		}
	})
}

func TestValidatePath_DirectoryPath(t *testing.T) {
	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks on tempdir: %v", err)
	}

	dirToValidate := filepath.Join(tempDir, "some_dir")
	if err := os.MkdirAll(dirToValidate, 0o755); err != nil {
		t.Fatalf("failed to mkdir dirToValidate: %v", err)
	}

	s, err := New([]string{tempDir}, true)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	_, err = s.ValidatePath(dirToValidate)
	if err == nil {
		t.Fatalf("expected directory path to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "path is a directory, expected a PDF file") {
		t.Errorf("expected 'path is a directory, expected a PDF file', got: %v", err)
	}
	if !strings.Contains(err.Error(), "cwd=") || !strings.Contains(err.Error(), "allowed_roots=") || !strings.Contains(err.Error(), "allowed_files=") {
		t.Errorf("expected status info in error: %v", err)
	}
}
