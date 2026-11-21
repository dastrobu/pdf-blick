package pdf

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A minimal valid 2-page PDF (Page 1: A4, Page 2: US Letter) with metadata and bookmark.
const samplePDF = `%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R /Outlines 5 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595.28 841.89] >>
endobj
4 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>
endobj
5 0 obj
<< /Type /Outlines /First 6 0 R /Last 6 0 R >>
endobj
6 0 obj
<< /Title (Cover Page) /Parent 5 0 R /Dest [3 0 R /XYZ 0 841.89 0] >>
endobj
7 0 obj
<< /Title (A Very Long Title For Testing Truncation) >>
endobj
xref
0 8
0000000000 65535 f 
0000000009 00000 n 
0000000074 00000 n 
0000000136 00000 n 
0000000217 00000 n 
0000000291 00000 n 
0000000351 00000 n 
0000000436 00000 n 
trailer
<< /Size 8 /Root 1 0 R /Info 7 0 R >>
startxref
507
%%EOF
`

func writeTestPDF(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "test.pdf")
	if err := os.WriteFile(pdfPath, []byte(samplePDF), 0o644); err != nil {
		t.Fatalf("failed to write test PDF: %v", err)
	}
	return pdfPath
}

func writeLargeTestPDF(t *testing.T, numPages int) string {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [")
	for i := 1; i <= numPages; i++ {
		fmt.Fprintf(&b, "%d 0 R ", i+2)
	}
	fmt.Fprintf(&b, "] /Count %d >>\nendobj\n", numPages)
	for i := 1; i <= numPages; i++ {
		fmt.Fprintf(&b, "%d 0 R obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n", i+2)
	}
	b.WriteString("xref\n0 1\n0000000000 65535 f \ntrailer\n<< /Root 1 0 R >>\nstartxref\n0\n%%EOF\n")

	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "large.pdf")
	if err := os.WriteFile(pdfPath, b.Bytes(), 0o644); err != nil {
		t.Fatalf("failed to write large test PDF: %v", err)
	}
	return pdfPath
}

func TestDetectPageFormat(t *testing.T) {
	tests := []struct {
		name     string
		widthPt  float64
		heightPt float64
		expected string
	}{
		{"A0 portrait", 2383.94, 3370.39, "A0"},
		{"A0 landscape", 3370.39, 2383.94, "A0"},
		{"A1", 1683.78, 2383.94, "A1"},
		{"A2", 1190.55, 1683.78, "A2"},
		{"A3", 841.89, 1190.55, "A3"},
		{"A4", 595.28, 841.89, "A4"},
		{"A5", 419.53, 595.28, "A5"},
		{"A6", 297.64, 419.53, "A6"},
		{"B4", 708.66, 1000.63, "B4"},
		{"B5", 498.90, 708.66, "B5"},
		{"US Letter", 612.0, 792.0, "US Letter"},
		{"US Legal", 612.0, 1008.0, "US Legal"},
		{"Tabloid", 792.0, 1224.0, "Tabloid"},
		{"16:9 Presentation 540x960", 960.0, 540.0, "16:9 Presentation"},
		{"16:9 Presentation 405x720", 720.0, 405.0, "16:9 Presentation"},
		{"16:9 Presentation aspect ratio", 1920.0, 1080.0, "16:9 Presentation"},
		{"4:3 Presentation", 720.0, 540.0, "4:3 Presentation"},
		{"custom square", 300.0, 300.0, "custom"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectPageFormat(tc.widthPt, tc.heightPt)
			if got != tc.expected {
				t.Errorf("detectPageFormat(%f, %f) = %q, want %q", tc.widthPt, tc.heightPt, got, tc.expected)
			}
		})
	}
}

func TestNewEngineWithConfig(t *testing.T) {
	// 1. Normalization of zero and negative fields
	engDefault := NewEngineWithConfig(Config{})
	cfg := engDefault.Config()
	if cfg.DefaultDPI != DefaultDPI {
		t.Errorf("expected DefaultDPI %d, got %d", DefaultDPI, cfg.DefaultDPI)
	}
	if cfg.MinDPI != DefaultMinDPI {
		t.Errorf("expected MinDPI %d, got %d", DefaultMinDPI, cfg.MinDPI)
	}
	if cfg.MaxDPI != DefaultMaxDPI {
		t.Errorf("expected MaxDPI %d, got %d", DefaultMaxDPI, cfg.MaxDPI)
	}
	if cfg.MaxImageDimension != DefaultMaxImageDimension {
		t.Errorf("expected MaxImageDimension %d, got %d", DefaultMaxImageDimension, cfg.MaxImageDimension)
	}
	if cfg.DefaultMaxListPages != DefaultMaxListPages {
		t.Errorf("expected DefaultMaxListPages %d, got %d", DefaultMaxListPages, cfg.DefaultMaxListPages)
	}
	if cfg.MaxListPagesWindow != DefaultMaxListPagesWindow {
		t.Errorf("expected MaxListPagesWindow %d, got %d", DefaultMaxListPagesWindow, cfg.MaxListPagesWindow)
	}
	if cfg.MaxBookmarks != DefaultMaxBookmarks {
		t.Errorf("expected MaxBookmarks %d, got %d", DefaultMaxBookmarks, cfg.MaxBookmarks)
	}
	if cfg.MaxAttachments != DefaultMaxAttachments {
		t.Errorf("expected MaxAttachments %d, got %d", DefaultMaxAttachments, cfg.MaxAttachments)
	}
	if cfg.MaxMetadataValueLen != DefaultMaxMetadataValueLen {
		t.Errorf("expected MaxMetadataValueLen %d, got %d", DefaultMaxMetadataValueLen, cfg.MaxMetadataValueLen)
	}

	// 2. Normalization of inverted MinDPI / MaxDPI
	engSwapped := NewEngineWithConfig(Config{
		MinDPI: 500,
		MaxDPI: 100,
	})
	cfgSwapped := engSwapped.Config()
	if cfgSwapped.MinDPI > cfgSwapped.MaxDPI {
		t.Errorf("expected MinDPI <= MaxDPI, got %d > %d", cfgSwapped.MinDPI, cfgSwapped.MaxDPI)
	}

	pdfPath := writeTestPDF(t)
	ctx := context.Background()

	// 3. Custom MaxImageDimension
	engSmallDim := NewEngineWithConfig(Config{
		MaxImageDimension: 200,
	})
	defer engSmallDim.Close()

	resDim, err := engSmallDim.RenderPage(ctx, RenderOptions{
		Path: pdfPath,
		Page: 1,
		DPI:  new(300),
	})
	if err != nil {
		t.Fatalf("RenderPage with small MaxImageDimension failed: %v", err)
	}
	if resDim.WidthPx > 200 || resDim.HeightPx > 200 {
		t.Errorf("expected dimensions <= 200, got %dx%d", resDim.WidthPx, resDim.HeightPx)
	}

	// 4. Custom DefaultDPI
	engCustomDPI := NewEngineWithConfig(Config{
		DefaultDPI: 200,
		MinDPI:     50,
		MaxDPI:     400,
	})
	defer engCustomDPI.Close()

	resDPI, err := engCustomDPI.RenderPage(ctx, RenderOptions{
		Path: pdfPath,
		Page: 1,
	})
	if err != nil {
		t.Fatalf("RenderPage with custom DefaultDPI failed: %v", err)
	}
	if resDPI.EffectiveDPI != 200 {
		t.Errorf("expected EffectiveDPI 200, got %d", resDPI.EffectiveDPI)
	}

	// 5. Custom MaxMetadataValueLen
	engTrunc := NewEngineWithConfig(Config{
		MaxMetadataValueLen: 11,
	})
	defer engTrunc.Close()

	resMeta, err := engTrunc.ListPages(ctx, pdfPath)
	if err != nil {
		t.Fatalf("ListPages with custom MaxMetadataValueLen failed: %v", err)
	}
	metadata, ok := resMeta["metadata"].(map[string]string)
	if !ok {
		t.Fatalf("expected metadata map, got %v", resMeta["metadata"])
	}
	if title, exists := metadata["title"]; exists {
		expectedPrefix := "A Very Long... (truncated)"
		if title != expectedPrefix {
			t.Errorf("expected truncated title %q, got %q", expectedPrefix, title)
		}
	}
}

func TestListPages(t *testing.T) {
	eng := NewEngine()
	defer eng.Close()

	pdfPath := writeTestPDF(t)
	ctx := context.Background()

	data, err := eng.ListPages(ctx, pdfPath)
	if err != nil {
		t.Fatalf("ListPages failed: %v", err)
	}

	if data["path"] != pdfPath {
		t.Errorf("got path %v, want %v", data["path"], pdfPath)
	}
	if data["page_count"] != 2 {
		t.Errorf("got page_count %v, want 2", data["page_count"])
	}
	if data["is_encrypted"] != false {
		t.Errorf("expected is_encrypted false, got %v", data["is_encrypted"])
	}

	// Unencrypted with full permissions should omit the permissions map
	if _, ok := data["permissions"]; ok {
		t.Errorf("expected permissions map to be omitted for unencrypted PDF with full permissions, but was present")
	}

	// Verify page_layout deduplication
	layout, ok := data["page_layout"].(map[string]any)
	if !ok {
		t.Fatalf("expected page_layout map, got %v", data["page_layout"])
	}
	if layout["format"] != "A4" {
		t.Errorf("expected page_layout format A4, got %v", layout["format"])
	}
	if layout["orientation"] != "portrait" {
		t.Errorf("expected page_layout orientation portrait, got %v", layout["orientation"])
	}

	pages, ok := data["pages"].([]map[string]any)
	if !ok || len(pages) != 2 {
		t.Fatalf("expected 2 pages in data['pages'], got %v", data["pages"])
	}

	// Page 1: Matches default layout (A4), so geometry fields must be omitted
	p1 := pages[0]
	if p1["page"] != 1 {
		t.Errorf("expected page 1, got %v", p1["page"])
	}
	if _, exists := p1["format"]; exists {
		t.Errorf("expected Page 1 to omit format because it matches page_layout, but found %v", p1["format"])
	}
	if _, exists := p1["width_pt"]; exists {
		t.Errorf("expected Page 1 to omit width_pt because it matches page_layout, but found %v", p1["width_pt"])
	}
	if _, exists := p1["height_pt"]; exists {
		t.Errorf("expected Page 1 to omit height_pt because it matches page_layout, but found %v", p1["height_pt"])
	}
	if _, exists := p1["orientation"]; exists {
		t.Errorf("expected Page 1 to omit orientation because it matches page_layout, but found %v", p1["orientation"])
	}

	// Page 2: US Letter, differs from page_layout, so geometry fields must be present
	p2 := pages[1]
	if p2["page"] != 2 {
		t.Errorf("expected page 2, got %v", p2["page"])
	}
	if p2["format"] != "US Letter" {
		t.Errorf("expected Page 2 format US Letter, got %v", p2["format"])
	}
	if p2["orientation"] != "portrait" {
		t.Errorf("expected Page 2 orientation portrait, got %v", p2["orientation"])
	}
	if p2["width_pt"] != 612.0 || p2["height_pt"] != 792.0 {
		t.Errorf("expected Page 2 612x792, got %vx%v", p2["width_pt"], p2["height_pt"])
	}

	// Check bookmarks
	bms, ok := data["bookmarks"].([]map[string]any)
	if !ok || len(bms) != 1 {
		t.Fatalf("expected 1 bookmark, got %v", data["bookmarks"])
	}
	if bms[0]["title"] != "Cover Page" {
		t.Errorf("expected bookmark title 'Cover Page', got %v", bms[0]["title"])
	}
	if bms[0]["page"] != 1 {
		t.Errorf("expected bookmark page 1, got %v", bms[0]["page"])
	}
}

func TestListPages_Pagination(t *testing.T) {
	eng := NewEngine()
	defer eng.Close()

	pdfPath := writeTestPDF(t)
	ctx := context.Background()

	// Inspect page 2 only
	data, err := eng.ListPages(ctx, pdfPath, ListPagesOptions{
		StartPage: new(2),
		EndPage:   new(2),
	})
	if err != nil {
		t.Fatalf("ListPages pagination failed: %v", err)
	}

	pages, ok := data["pages"].([]map[string]any)
	if !ok || len(pages) != 1 {
		t.Fatalf("expected 1 page returned, got %v", data["pages"])
	}
	if pages[0]["page"] != 2 {
		t.Errorf("expected page 2, got %v", pages[0]["page"])
	}
	if data["pages_returned"] != 1 {
		t.Errorf("expected pages_returned 1, got %v", data["pages_returned"])
	}
	if data["pages_truncated"] != true {
		t.Errorf("expected pages_truncated true, got %v", data["pages_truncated"])
	}
}

func TestListPages_PaginationCaps(t *testing.T) {
	eng := NewEngine()
	defer eng.Close()

	// Create a 250-page PDF to test default 100-page limit and 200-page window cap
	pdfPath := writeLargeTestPDF(t, 250)
	ctx := context.Background()

	// 1. Default pagination: StartPage=nil, EndPage=nil -> should default to 100 pages
	resDefault, err := eng.ListPages(ctx, pdfPath)
	if err != nil {
		t.Fatalf("ListPages with default pagination failed: %v", err)
	}
	pagesDef, ok := resDefault["pages"].([]map[string]any)
	if !ok || len(pagesDef) != DefaultMaxListPages {
		t.Fatalf("expected %d pages by default, got %d", DefaultMaxListPages, len(pagesDef))
	}
	if resDefault["pages_returned"] != DefaultMaxListPages {
		t.Errorf("expected pages_returned %d, got %v", DefaultMaxListPages, resDefault["pages_returned"])
	}
	if resDefault["pages_truncated"] != true {
		t.Errorf("expected pages_truncated true, got %v", resDefault["pages_truncated"])
	}

	// 2. Request window larger than MaxListPagesWindow (250 pages) -> capped at 200
	resCapped, err := eng.ListPages(ctx, pdfPath, ListPagesOptions{
		StartPage: new(1),
		EndPage:   new(250),
	})
	if err != nil {
		t.Fatalf("ListPages with large window failed: %v", err)
	}
	pagesCap, ok := resCapped["pages"].([]map[string]any)
	if !ok || len(pagesCap) != MaxListPagesWindow {
		t.Fatalf("expected %d pages due to MaxListPagesWindow cap, got %d", MaxListPagesWindow, len(pagesCap))
	}
	if resCapped["pages_returned"] != MaxListPagesWindow {
		t.Errorf("expected pages_returned %d, got %v", MaxListPagesWindow, resCapped["pages_returned"])
	}
	if resCapped["pages_truncated"] != true {
		t.Errorf("expected pages_truncated true, got %v", resCapped["pages_truncated"])
	}
}

func TestListPages_ValidationErrors(t *testing.T) {
	eng := NewEngine()
	defer eng.Close()

	pdfPath := writeTestPDF(t)
	ctx := context.Background()

	// StartPage < 1 (0 or negative)
	_, err := eng.ListPages(ctx, pdfPath, ListPagesOptions{StartPage: new(0)})
	if err == nil || !strings.Contains(err.Error(), "start_page must be >= 1") {
		t.Errorf("expected start_page >= 1 error for 0, got: %v", err)
	}

	_, err = eng.ListPages(ctx, pdfPath, ListPagesOptions{StartPage: new(-1)})
	if err == nil || !strings.Contains(err.Error(), "start_page must be >= 1") {
		t.Errorf("expected start_page >= 1 error for -1, got: %v", err)
	}

	// EndPage < 1 (0 or negative)
	_, err = eng.ListPages(ctx, pdfPath, ListPagesOptions{EndPage: new(0)})
	if err == nil || !strings.Contains(err.Error(), "end_page must be >= 1") {
		t.Errorf("expected end_page >= 1 error for 0, got: %v", err)
	}

	_, err = eng.ListPages(ctx, pdfPath, ListPagesOptions{EndPage: new(-1)})
	if err == nil || !strings.Contains(err.Error(), "end_page must be >= 1") {
		t.Errorf("expected end_page >= 1 error for -1, got: %v", err)
	}

	// StartPage > pageCount
	_, err = eng.ListPages(ctx, pdfPath, ListPagesOptions{StartPage: new(10)})
	if err == nil || !strings.Contains(err.Error(), "out of bounds") {
		t.Errorf("expected out of bounds error, got: %v", err)
	}

	// EndPage < StartPage
	_, err = eng.ListPages(ctx, pdfPath, ListPagesOptions{StartPage: new(2), EndPage: new(1)})
	if err == nil || !strings.Contains(err.Error(), "must be >=") {
		t.Errorf("expected end_page >= start_page error, got: %v", err)
	}
}

func TestRenderPage_FullPage(t *testing.T) {
	eng := NewEngine()
	defer eng.Close()

	pdfPath := writeTestPDF(t)
	ctx := context.Background()

	res, err := eng.RenderPage(ctx, RenderOptions{
		Path: pdfPath,
		Page: 1,
	})
	if err != nil {
		t.Fatalf("RenderPage failed: %v", err)
	}

	if res.Page != 1 {
		t.Errorf("expected page 1, got %d", res.Page)
	}
	if res.TotalPages != 2 {
		t.Errorf("expected total pages 2, got %d", res.TotalPages)
	}
	if res.EffectiveDPI != 150 {
		t.Errorf("expected effective DPI 150, got %d", res.EffectiveDPI)
	}
	if len(res.PNGBytes) == 0 {
		t.Fatal("expected non-empty PNG bytes")
	}

	img, err := png.Decode(bytes.NewReader(res.PNGBytes))
	if err != nil {
		t.Fatalf("failed to decode rendered PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != res.WidthPx || bounds.Dy() != res.HeightPx {
		t.Errorf("PNG bounds (%dx%d) do not match result dimensions (%dx%d)",
			bounds.Dx(), bounds.Dy(), res.WidthPx, res.HeightPx)
	}
}

func TestRenderPage_CustomDPI(t *testing.T) {
	eng := NewEngine()
	defer eng.Close()

	pdfPath := writeTestPDF(t)
	ctx := context.Background()

	res150, err := eng.RenderPage(ctx, RenderOptions{
		Path: pdfPath,
		Page: 1,
		DPI:  new(150),
	})
	if err != nil {
		t.Fatalf("RenderPage at 150 DPI failed: %v", err)
	}

	res300, err := eng.RenderPage(ctx, RenderOptions{
		Path: pdfPath,
		Page: 1,
		DPI:  new(300),
	})
	if err != nil {
		t.Fatalf("RenderPage at 300 DPI failed: %v", err)
	}

	if res300.EffectiveDPI != 300 {
		t.Errorf("expected EffectiveDPI 300, got %d", res300.EffectiveDPI)
	}

	// Width and height should roughly double
	ratioW := float64(res300.WidthPx) / float64(res150.WidthPx)
	if ratioW < 1.9 || ratioW > 2.1 {
		t.Errorf("expected ~2x width scale, got ratio %f", ratioW)
	}
}

func TestRenderPage_CropAutoDPI(t *testing.T) {
	eng := NewEngine()
	defer eng.Close()

	pdfPath := writeTestPDF(t)
	ctx := context.Background()

	left := 0.25
	top := 0.25
	right := 0.75
	bottom := 0.75

	res, err := eng.RenderPage(ctx, RenderOptions{
		Path:   pdfPath,
		Page:   1,
		Left:   &left,
		Top:    &top,
		Right:  &right,
		Bottom: &bottom,
	})
	if err != nil {
		t.Fatalf("RenderPage with crop failed: %v", err)
	}

	// With crop width/height 0.5, effective DPI should scale up to 150 / 0.5 = 300
	if res.EffectiveDPI != 300 {
		t.Errorf("expected auto-scaled DPI 300, got %d", res.EffectiveDPI)
	}

	img, err := png.Decode(bytes.NewReader(res.PNGBytes))
	if err != nil {
		t.Fatalf("failed to decode rendered PNG: %v", err)
	}
	if img.Bounds().Dx() != res.WidthPx || img.Bounds().Dy() != res.HeightPx {
		t.Errorf("decoded image size does not match result dimensions")
	}

	// 1D slice: top=0.5, with left, right, bottom omitted
	top1D := 0.5
	res1D, err := eng.RenderPage(ctx, RenderOptions{
		Path: pdfPath,
		Page: 1,
		Top:  &top1D,
	})
	if err != nil {
		t.Fatalf("RenderPage with 1D slice crop failed: %v", err)
	}
	if res1D.EffectiveDPI != 300 {
		t.Errorf("expected 1D slice auto-scaled DPI 300, got %d", res1D.EffectiveDPI)
	}
	if res1D.Top != 0.5 || res1D.Bottom != 1.0 || res1D.Left != 0.0 || res1D.Right != 1.0 {
		t.Errorf("unexpected crop bounds on 1D result: left=%f, top=%f, right=%f, bottom=%f",
			res1D.Left, res1D.Top, res1D.Right, res1D.Bottom)
	}
}

func TestRenderPage_DPIClampingAndSafetyCap(t *testing.T) {
	eng := NewEngine()
	defer eng.Close()

	pdfPath := writeTestPDF(t)
	ctx := context.Background()

	// High DPI (600) on A4 (595.28 x 841.89 pt) would be ~4960 x ~7016 px, exceeding 4096px cap
	res, err := eng.RenderPage(ctx, RenderOptions{
		Path: pdfPath,
		Page: 1,
		DPI:  new(600),
	})
	if err != nil {
		t.Fatalf("RenderPage with high DPI failed: %v", err)
	}

	if res.WidthPx > 4096 || res.HeightPx > 4096 {
		t.Errorf("expected dimensions <= 4096, got %dx%d", res.WidthPx, res.HeightPx)
	}

	// Low DPI below 36 should clamp to 36
	resLow, err := eng.RenderPage(ctx, RenderOptions{
		Path: pdfPath,
		Page: 1,
		DPI:  new(10),
	})
	if err != nil {
		t.Fatalf("RenderPage with low DPI failed: %v", err)
	}
	if resLow.EffectiveDPI != 36 {
		t.Errorf("expected DPI clamped to 36, got %d", resLow.EffectiveDPI)
	}
}

func TestRenderPage_ValidationErrors(t *testing.T) {
	eng := NewEngine()
	defer eng.Close()

	pdfPath := writeTestPDF(t)
	ctx := context.Background()

	// Invalid page < 1
	_, err := eng.RenderPage(ctx, RenderOptions{Path: pdfPath, Page: 0})
	if err == nil {
		t.Error("expected error for page 0, got nil")
	}

	// Invalid page > total pages
	_, err = eng.RenderPage(ctx, RenderOptions{Path: pdfPath, Page: 99})
	if err == nil {
		t.Error("expected error for page 99, got nil")
	}

	// Invalid DPI: 0 or negative
	_, err = eng.RenderPage(ctx, RenderOptions{Path: pdfPath, Page: 1, DPI: new(0)})
	if err == nil || !strings.Contains(err.Error(), "dpi must be > 0") {
		t.Errorf("expected 'dpi must be > 0' for DPI 0, got: %v", err)
	}

	_, err = eng.RenderPage(ctx, RenderOptions{Path: pdfPath, Page: 1, DPI: new(-10)})
	if err == nil || !strings.Contains(err.Error(), "dpi must be > 0") {
		t.Errorf("expected 'dpi must be > 0' for DPI -10, got: %v", err)
	}

	// Invalid crop: left >= right
	left := 0.8
	right := 0.2
	_, err = eng.RenderPage(ctx, RenderOptions{
		Path:  pdfPath,
		Page:  1,
		Left:  &left,
		Right: &right,
	})
	if err == nil {
		t.Error("expected error for left >= right, got nil")
	}

	// Invalid crop: negative coordinate
	neg := -0.1
	_, err = eng.RenderPage(ctx, RenderOptions{
		Path: pdfPath,
		Page: 1,
		Top:  &neg,
	})
	if err == nil {
		t.Error("expected error for top < 0, got nil")
	}

	// Invalid crop: coordinate > 1.0
	over := 1.1
	_, err = eng.RenderPage(ctx, RenderOptions{
		Path:   pdfPath,
		Page:   1,
		Bottom: &over,
	})
	if err == nil {
		t.Error("expected error for bottom > 1.0, got nil")
	}
}

func TestEngine_WarmupAndClose(t *testing.T) {
	eng := NewEngine()
	eng.Warmup()

	// Give warmup a moment to run in background
	time.Sleep(100 * time.Millisecond)

	pdfPath := writeTestPDF(t)
	ctx := context.Background()

	// Operations should succeed after Warmup
	res, err := eng.ListPages(ctx, pdfPath)
	if err != nil {
		t.Fatalf("ListPages failed after Warmup: %v", err)
	}
	if res["page_count"] != 2 {
		t.Fatalf("expected 2 pages, got %v", res["page_count"])
	}

	// Close engine
	if err := eng.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Subsequent calls should fail because engine is closed
	_, err = eng.ListPages(ctx, pdfPath)
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("expected closed error, got: %v", err)
	}

	_, err = eng.RenderPage(ctx, RenderOptions{Path: pdfPath, Page: 1})
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("expected closed error, got: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	eng := NewEngine()
	defer eng.Close()

	pdfPath := writeTestPDF(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := eng.ListPages(ctx, pdfPath)
	if err == nil {
		t.Error("expected error with cancelled context in ListPages, got nil")
	}

	_, err = eng.RenderPage(ctx, RenderOptions{Path: pdfPath, Page: 1})
	if err == nil {
		t.Error("expected error with cancelled context in RenderPage, got nil")
	}
}

func TestEngine_A4LetterFixture(t *testing.T) {
	pdfPath := filepath.Join("..", "..", "testdata", "A4 Letter.pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		t.Skipf("testdata/A4 Letter.pdf not found: %v", err)
	}

	eng := NewEngine()
	defer eng.Close()
	ctx := context.Background()

	data, err := eng.ListPages(ctx, pdfPath)
	if err != nil {
		t.Fatalf("ListPages failed: %v", err)
	}
	if data["page_count"] != 1 {
		t.Errorf("expected page_count 1, got %v", data["page_count"])
	}

	meta, ok := data["metadata"].(map[string]string)
	if !ok {
		t.Fatalf("expected metadata map, got %T: %v", data["metadata"], data["metadata"])
	}
	if meta["title"] != "A4 Letter" {
		t.Errorf("expected metadata title 'A4 Letter', got %v", meta["title"])
	}

	layout, ok := data["page_layout"].(map[string]any)
	if !ok {
		t.Fatalf("expected page_layout map, got %v", data["page_layout"])
	}
	if layout["format"] != "A4" {
		t.Errorf("expected format A4, got %v", layout["format"])
	}
	if layout["orientation"] != "portrait" {
		t.Errorf("expected orientation portrait, got %v", layout["orientation"])
	}

	// Render full page at default DPI (150)
	res, err := eng.RenderPage(ctx, RenderOptions{Path: pdfPath, Page: 1})
	if err != nil {
		t.Fatalf("RenderPage failed: %v", err)
	}
	if res.Page != 1 {
		t.Errorf("expected page 1, got %d", res.Page)
	}
	if res.TotalPages != 1 {
		t.Errorf("expected total pages 1, got %d", res.TotalPages)
	}
	if res.WidthPx != 1241 || res.HeightPx != 1754 {
		t.Errorf("expected 1241x1754, got %dx%d", res.WidthPx, res.HeightPx)
	}
	img, err := png.Decode(bytes.NewReader(res.PNGBytes))
	if err != nil {
		t.Fatalf("failed to decode rendered PNG: %v", err)
	}
	if img.Bounds().Dx() != 1241 || img.Bounds().Dy() != 1754 {
		t.Errorf("expected image bounds 1241x1754, got %v", img.Bounds())
	}
}
