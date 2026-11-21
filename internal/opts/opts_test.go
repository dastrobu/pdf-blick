package opts

import (
	"reflect"
	"testing"

	"github.com/dastrobu/pdf-mcp/internal/tools"
)

func TestParseRunDefaults(t *testing.T) {
	options, _, err := ParseArgs([]string{"run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(options.Run.Roots) != 0 {
		t.Fatalf("expected empty roots by default, got %v", options.Run.Roots)
	}
	if options.Run.NoWorkdir {
		t.Fatalf("expected NoWorkdir false by default")
	}
	if options.Run.Debug {
		t.Fatalf("expected Debug false by default")
	}

	// Verify EngineOpts defaults
	cfg := options.Run.EngineOpts
	if cfg.DefaultDPI != 150 {
		t.Errorf("expected DefaultDPI 150, got %d", cfg.DefaultDPI)
	}
	if cfg.MinDPI != 36 {
		t.Errorf("expected MinDPI 36, got %d", cfg.MinDPI)
	}
	if cfg.MaxDPI != 600 {
		t.Errorf("expected MaxDPI 600, got %d", cfg.MaxDPI)
	}
	if cfg.MaxImageDimension != 4096 {
		t.Errorf("expected MaxImageDimension 4096, got %d", cfg.MaxImageDimension)
	}
	if cfg.DefaultMaxPages != 100 {
		t.Errorf("expected DefaultMaxPages 100, got %d", cfg.DefaultMaxPages)
	}
	if cfg.MaxPagesWindow != 200 {
		t.Errorf("expected MaxPagesWindow 200, got %d", cfg.MaxPagesWindow)
	}
	if cfg.MaxBookmarks != 200 {
		t.Errorf("expected MaxBookmarks 200, got %d", cfg.MaxBookmarks)
	}
	if cfg.MaxAttachments != 50 {
		t.Errorf("expected MaxAttachments 50, got %d", cfg.MaxAttachments)
	}
	if cfg.MaxMetadataLen != 512 {
		t.Errorf("expected MaxMetadataLen 512, got %d", cfg.MaxMetadataLen)
	}

	pdfCfg := cfg.ToPDFConfig()
	if pdfCfg.DefaultDPI != 150 || pdfCfg.MinDPI != 36 || pdfCfg.MaxDPI != 600 ||
		pdfCfg.MaxImageDimension != 4096 || pdfCfg.DefaultMaxListPages != 100 ||
		pdfCfg.MaxListPagesWindow != 200 || pdfCfg.MaxBookmarks != 200 ||
		pdfCfg.MaxAttachments != 50 || pdfCfg.MaxMetadataValueLen != 512 {
		t.Errorf("ToPDFConfig mismatched defaults: %+v", pdfCfg)
	}
}

func TestParseRunFlags(t *testing.T) {
	args := []string{
		"run",
		"--root", "/path/one",
		"-r", "/path/two",
		"--no-workdir",
		"--debug",
		"--default-dpi", "200",
		"--min-dpi", "50",
		"--max-dpi", "800",
		"--max-image-dimension", "2048",
		"--default-max-pages", "50",
		"--max-pages-window", "150",
		"--max-bookmarks", "100",
		"--max-attachments", "25",
		"--max-metadata-len", "256",
	}

	options, _, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedRoots := []string{"/path/one", "/path/two"}
	if !reflect.DeepEqual(options.Run.Roots, expectedRoots) {
		t.Fatalf("expected roots %v, got %v", expectedRoots, options.Run.Roots)
	}
	if !options.Run.NoWorkdir {
		t.Fatalf("expected NoWorkdir to be true")
	}
	if !options.Run.Debug {
		t.Fatalf("expected Debug to be true")
	}

	eng := options.Run.EngineOpts
	if eng.DefaultDPI != 200 {
		t.Errorf("expected DefaultDPI 200, got %d", eng.DefaultDPI)
	}
	if eng.MinDPI != 50 {
		t.Errorf("expected MinDPI 50, got %d", eng.MinDPI)
	}
	if eng.MaxDPI != 800 {
		t.Errorf("expected MaxDPI 800, got %d", eng.MaxDPI)
	}
	if eng.MaxImageDimension != 2048 {
		t.Errorf("expected MaxImageDimension 2048, got %d", eng.MaxImageDimension)
	}
	if eng.DefaultMaxPages != 50 {
		t.Errorf("expected DefaultMaxPages 50, got %d", eng.DefaultMaxPages)
	}
	if eng.MaxPagesWindow != 150 {
		t.Errorf("expected MaxPagesWindow 150, got %d", eng.MaxPagesWindow)
	}
	if eng.MaxBookmarks != 100 {
		t.Errorf("expected MaxBookmarks 100, got %d", eng.MaxBookmarks)
	}
	if eng.MaxAttachments != 25 {
		t.Errorf("expected MaxAttachments 25, got %d", eng.MaxAttachments)
	}
	if eng.MaxMetadataLen != 256 {
		t.Errorf("expected MaxMetadataLen 256, got %d", eng.MaxMetadataLen)
	}
}

func TestParseRunEnvVars(t *testing.T) {
	t.Setenv("PDF_MCP_ROOT", "/env/dir1:/env/dir2")
	t.Setenv("PDF_MCP_NO_WORKDIR", "true")
	t.Setenv("PDF_MCP_DEBUG", "true")
	t.Setenv("PDF_MCP_DEFAULT_DPI", "180")
	t.Setenv("PDF_MCP_MIN_DPI", "45")
	t.Setenv("PDF_MCP_MAX_DPI", "720")
	t.Setenv("PDF_MCP_MAX_IMAGE_DIMENSION", "3000")
	t.Setenv("PDF_MCP_DEFAULT_MAX_PAGES", "80")
	t.Setenv("PDF_MCP_MAX_PAGES_WINDOW", "160")
	t.Setenv("PDF_MCP_MAX_BOOKMARKS", "120")
	t.Setenv("PDF_MCP_MAX_ATTACHMENTS", "30")
	t.Setenv("PDF_MCP_MAX_METADATA_LEN", "300")

	options, _, err := ParseArgs([]string{"run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedRoots := []string{"/env/dir1", "/env/dir2"}
	if !reflect.DeepEqual(options.Run.Roots, expectedRoots) {
		t.Fatalf("expected roots from env %v, got %v", expectedRoots, options.Run.Roots)
	}
	if !options.Run.NoWorkdir {
		t.Fatalf("expected NoWorkdir to be true from env")
	}
	if !options.Run.Debug {
		t.Fatalf("expected Debug to be true from env")
	}

	eng := options.Run.EngineOpts
	if eng.DefaultDPI != 180 {
		t.Errorf("expected DefaultDPI 180, got %d", eng.DefaultDPI)
	}
	if eng.MinDPI != 45 {
		t.Errorf("expected MinDPI 45, got %d", eng.MinDPI)
	}
	if eng.MaxDPI != 720 {
		t.Errorf("expected MaxDPI 720, got %d", eng.MaxDPI)
	}
	if eng.MaxImageDimension != 3000 {
		t.Errorf("expected MaxImageDimension 3000, got %d", eng.MaxImageDimension)
	}
	if eng.DefaultMaxPages != 80 {
		t.Errorf("expected DefaultMaxPages 80, got %d", eng.DefaultMaxPages)
	}
	if eng.MaxPagesWindow != 160 {
		t.Errorf("expected MaxPagesWindow 160, got %d", eng.MaxPagesWindow)
	}
	if eng.MaxBookmarks != 120 {
		t.Errorf("expected MaxBookmarks 120, got %d", eng.MaxBookmarks)
	}
	if eng.MaxAttachments != 30 {
		t.Errorf("expected MaxAttachments 30, got %d", eng.MaxAttachments)
	}
	if eng.MaxMetadataLen != 300 {
		t.Errorf("expected MaxMetadataLen 300, got %d", eng.MaxMetadataLen)
	}
}

func TestParseToolListPages(t *testing.T) {
	args := []string{
		"tool",
		"list_pages",
		"--root", "/allowed/dir",
		"--no-workdir",
		"--path", "sample.pdf",
		"--start-page", "5",
		"--end-page", "25",
		"--default-max-pages", "50",
	}

	options, _, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedRoots := []string{"/allowed/dir"}
	if !reflect.DeepEqual(options.Tool.ListPages.Roots, expectedRoots) {
		t.Fatalf("expected roots %v, got %v", expectedRoots, options.Tool.ListPages.Roots)
	}
	if !options.Tool.ListPages.NoWorkdir {
		t.Fatalf("expected NoWorkdir to be true")
	}
	if options.Tool.ListPages.Path != "sample.pdf" {
		t.Fatalf("expected path sample.pdf, got %s", options.Tool.ListPages.Path)
	}
	if options.Tool.ListPages.StartPage == nil || *options.Tool.ListPages.StartPage != 5 {
		t.Fatalf("expected StartPage 5, got %v", options.Tool.ListPages.StartPage)
	}
	if options.Tool.ListPages.EndPage == nil || *options.Tool.ListPages.EndPage != 25 {
		t.Fatalf("expected EndPage 25, got %v", options.Tool.ListPages.EndPage)
	}
	if options.Tool.ListPages.DefaultMaxPages != 50 {
		t.Fatalf("expected DefaultMaxPages 50, got %d", options.Tool.ListPages.DefaultMaxPages)
	}

	// Verify omitted start-page and end-page default to nil
	optsDefault, _, err := ParseArgs([]string{"tool", "list_pages", "--path", "sample.pdf"})
	if err != nil {
		t.Fatalf("unexpected error parsing defaults: %v", err)
	}
	if optsDefault.Tool.ListPages.StartPage != nil {
		t.Fatalf("expected nil StartPage when omitted, got %v", optsDefault.Tool.ListPages.StartPage)
	}
	if optsDefault.Tool.ListPages.EndPage != nil {
		t.Fatalf("expected nil EndPage when omitted, got %v", optsDefault.Tool.ListPages.EndPage)
	}
}

func TestParseToolGetPage(t *testing.T) {
	args := []string{
		"tool",
		"get_page",
		"--root", "/docs",
		"--path", "doc.pdf",
		"--page", "3",
		"--dpi", "300",
		"--left", "0.1",
		"--top", "0.2",
		"--right", "0.85",
		"--bottom", "0.95",
		"--max-image-dimension", "2048",
		"-o", "output.png",
	}

	options, _, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmd := options.Tool.GetPage
	if !reflect.DeepEqual(cmd.Roots, []string{"/docs"}) {
		t.Fatalf("expected roots [/docs], got %v", cmd.Roots)
	}
	if cmd.Path != "doc.pdf" {
		t.Fatalf("expected path doc.pdf, got %s", cmd.Path)
	}
	if cmd.Page != 3 {
		t.Fatalf("expected page 3, got %d", cmd.Page)
	}
	if cmd.DPI == nil || *cmd.DPI != 300 {
		t.Fatalf("expected DPI 300, got %v", cmd.DPI)
	}
	if cmd.Left == nil || *cmd.Left != 0.1 {
		t.Fatalf("expected Left 0.1, got %v", cmd.Left)
	}
	if cmd.Top == nil || *cmd.Top != 0.2 {
		t.Fatalf("expected Top 0.2, got %v", cmd.Top)
	}
	if cmd.Right == nil || *cmd.Right != 0.85 {
		t.Fatalf("expected Right 0.85, got %v", cmd.Right)
	}
	if cmd.Bottom == nil || *cmd.Bottom != 0.95 {
		t.Fatalf("expected Bottom 0.95, got %v", cmd.Bottom)
	}
	if cmd.MaxImageDimension != 2048 {
		t.Fatalf("expected MaxImageDimension 2048, got %d", cmd.MaxImageDimension)
	}
	if cmd.Output != "output.png" {
		t.Fatalf("expected output output.png, got %s", cmd.Output)
	}

	// Verify omitted DPI and crop coordinates default to nil
	optsDefault, _, err := ParseArgs([]string{"tool", "get_page", "--path", "doc.pdf", "--page", "1"})
	if err != nil {
		t.Fatalf("unexpected error parsing defaults: %v", err)
	}
	if optsDefault.Tool.GetPage.DPI != nil {
		t.Fatalf("expected nil DPI when omitted, got %v", optsDefault.Tool.GetPage.DPI)
	}
	if optsDefault.Tool.GetPage.Left != nil || optsDefault.Tool.GetPage.Top != nil || optsDefault.Tool.GetPage.Right != nil || optsDefault.Tool.GetPage.Bottom != nil {
		t.Fatalf("expected nil crop coordinates when omitted")
	}
}

func TestParseVersionFlag(t *testing.T) {
	options, _, err := ParseArgs([]string{"--version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !options.Version {
		t.Fatalf("expected Version flag to be true")
	}
}

func TestExecuteHandlers(t *testing.T) {
	runExecuted := false
	runCmd := RunCmd{
		Handler: func() error {
			runExecuted = true
			return nil
		},
	}
	if err := runCmd.Execute(nil); err != nil || !runExecuted {
		t.Fatalf("RunCmd execute failed: err=%v, executed=%v", err, runExecuted)
	}

	completionExecuted := false
	compCmd := CompletionBashCmd{
		Handler: func() error {
			completionExecuted = true
			return nil
		},
	}
	if err := compCmd.Execute(nil); err != nil || !completionExecuted {
		t.Fatalf("CompletionBashCmd execute failed: err=%v, executed=%v", err, completionExecuted)
	}

	listPagesExecuted := false
	listCmd := ListPagesCmd{
		SandboxOpts: SandboxOpts{Roots: []string{"/test"}},
		EngineOpts:  EngineOpts{MaxBookmarks: 150},
		ListPagesInput: tools.ListPagesInput{
			Path: "test.pdf",
		},
		Handler: func(sb SandboxOpts, eng EngineOpts, in tools.ListPagesInput) error {
			listPagesExecuted = true
			if len(sb.Roots) != 1 || sb.Roots[0] != "/test" || in.Path != "test.pdf" || eng.MaxBookmarks != 150 {
				t.Fatalf("unexpected input in handler: sb=%v, eng=%v, in=%v", sb, eng, in)
			}
			return nil
		},
	}
	if err := listCmd.Execute(nil); err != nil || !listPagesExecuted {
		t.Fatalf("ListPagesCmd execute failed: err=%v, executed=%v", err, listPagesExecuted)
	}

	getPageExecuted := false
	getPageCmd := GetPageCmd{
		SandboxOpts: SandboxOpts{Roots: []string{"/test"}},
		EngineOpts:  EngineOpts{MaxImageDimension: 2048},
		GetPageInput: tools.GetPageInput{
			Path: "test.pdf",
			Page: 1,
		},
		Output: "out.png",
		Handler: func(sb SandboxOpts, eng EngineOpts, in tools.GetPageInput, out string) error {
			getPageExecuted = true
			if out != "out.png" || in.Page != 1 || eng.MaxImageDimension != 2048 {
				t.Fatalf("unexpected input in handler: in=%v, eng=%v, out=%s", in, eng, out)
			}
			return nil
		},
	}
	if err := getPageCmd.Execute(nil); err != nil || !getPageExecuted {
		t.Fatalf("GetPageCmd execute failed: err=%v, executed=%v", err, getPageExecuted)
	}
}
