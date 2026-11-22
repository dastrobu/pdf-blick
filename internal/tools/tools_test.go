package tools

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dastrobu/pdf-blick/internal/pdf"
	"github.com/dastrobu/pdf-blick/internal/sandbox"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewHandler(t *testing.T) {
	h := NewHandler(nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.Sandbox != nil || h.Engine != nil {
		t.Fatalf("expected nil sandbox and engine, got %v, %v", h.Sandbox, h.Engine)
	}
}

func TestRegisterAll(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)

	h := NewHandler(nil, nil)
	// RegisterAll should not panic and should register tools successfully
	RegisterAll(srv, h)
}

func TestHandleListPages_NilSandbox(t *testing.T) {
	h := NewHandler(nil, nil)
	_, _, err := h.HandleListPages(context.Background(), nil, ListPagesInput{Path: "test.pdf"})
	if err == nil {
		t.Fatal("expected error when sandbox is nil, got nil")
	}
	if !strings.Contains(err.Error(), "sandbox not initialized") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestHandleGetPage_NilSandbox(t *testing.T) {
	h := NewHandler(nil, nil)
	_, _, err := h.HandleGetPage(context.Background(), nil, GetPageInput{Path: "test.pdf", Page: 1})
	if err == nil {
		t.Fatal("expected error when sandbox is nil, got nil")
	}
	if !strings.Contains(err.Error(), "sandbox not initialized") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestStructTagsPlainDescriptions(t *testing.T) {
	// Verify jsonschema struct tags in ListPagesInput and GetPageInput are plain string descriptions,
	// never key=value pairs which cause modelcontextprotocol/go-sdk to panic.
	structs := []any{
		ListPagesInput{},
		GetPageInput{},
	}

	for _, s := range structs {
		val := reflect.TypeOf(s)
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			schemaTag := field.Tag.Get("jsonschema")
			if schemaTag != "" {
				if strings.Contains(schemaTag, "=") {
					t.Errorf("field %s.%s has key=value in jsonschema tag: %q", val.Name(), field.Name, schemaTag)
				}
			}
		}
	}
}

func TestListPagesInputJSONRoundTrip(t *testing.T) {
	startPage := 10
	endPage := 30
	in := ListPagesInput{
		Path:      "test.pdf",
		StartPage: &startPage,
		EndPage:   &endPage,
	}

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("failed to marshal ListPagesInput: %v", err)
	}

	var out ListPagesInput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("failed to unmarshal ListPagesInput: %v", err)
	}

	if out.Path != in.Path {
		t.Fatalf("mismatched Path: got %s, want %s", out.Path, in.Path)
	}
	if out.StartPage == nil || *out.StartPage != startPage {
		t.Fatalf("mismatched StartPage: got %v, want %v", out.StartPage, startPage)
	}
	if out.EndPage == nil || *out.EndPage != endPage {
		t.Fatalf("mismatched EndPage: got %v, want %v", out.EndPage, endPage)
	}

	// Verify omitted fields decode to nil
	emptyJSON := `{"path":"empty.pdf"}`
	var emptyOut ListPagesInput
	if err := json.Unmarshal([]byte(emptyJSON), &emptyOut); err != nil {
		t.Fatalf("failed to unmarshal empty JSON: %v", err)
	}
	if emptyOut.StartPage != nil || emptyOut.EndPage != nil {
		t.Fatalf("expected nil StartPage and EndPage when omitted, got %v, %v", emptyOut.StartPage, emptyOut.EndPage)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	dpi := 300
	rot := 180
	left := 0.1
	top := 0.2
	right := 0.8
	bottom := 0.9

	in := GetPageInput{
		Path:   "example.pdf",
		Page:   2,
		DPI:    &dpi,
		Rotate: &rot,
		Left:   &left,
		Top:    &top,
		Right:  &right,
		Bottom: &bottom,
	}

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("failed to marshal GetPageInput: %v", err)
	}

	var out GetPageInput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("failed to unmarshal GetPageInput: %v", err)
	}

	if out.Path != in.Path || out.Page != in.Page {
		t.Fatalf("mismatched basic fields: got %+v, want %+v", out, in)
	}
	if out.DPI == nil || *out.DPI != dpi {
		t.Fatalf("mismatched DPI: got %v, want %v", out.DPI, dpi)
	}
	if out.Rotate == nil || *out.Rotate != rot {
		t.Fatalf("mismatched Rotate: got %v, want %v", out.Rotate, rot)
	}
	if out.Left == nil || *out.Left != left {
		t.Fatalf("mismatched Left: got %v, want %v", out.Left, left)
	}
	if out.Top == nil || *out.Top != top {
		t.Fatalf("mismatched Top: got %v, want %v", out.Top, top)
	}
	if out.Right == nil || *out.Right != right {
		t.Fatalf("mismatched Right: got %v, want %v", out.Right, right)
	}
	if out.Bottom == nil || *out.Bottom != bottom {
		t.Fatalf("mismatched Bottom: got %v, want %v", out.Bottom, bottom)
	}

	// Verify omitted DPI and Rotate decode to nil
	emptyJSON := `{"path":"empty.pdf","page":1}`
	var emptyOut GetPageInput
	if err := json.Unmarshal([]byte(emptyJSON), &emptyOut); err != nil {
		t.Fatalf("failed to unmarshal empty JSON: %v", err)
	}
	if emptyOut.DPI != nil || emptyOut.Rotate != nil {
		t.Fatalf("expected nil DPI and Rotate when omitted, got %v, %v", emptyOut.DPI, emptyOut.Rotate)
	}
}

const sampleMinimalPDF = `%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R >>
endobj
4 0 obj
<< /Length 44 >>
stream
0.9 0.1 0.1 rg
50 50 200 200 re
f
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000198 00000 n 
trailer
<< /Size 5 /Root 1 0 R >>
startxref
292
%%EOF
`

func TestEndToEndMCPProtocol(t *testing.T) {
	allowedDir := t.TempDir()
	outsideDir := t.TempDir()

	pdfPath := filepath.Join(allowedDir, "doc.pdf")
	if err := os.WriteFile(pdfPath, []byte(sampleMinimalPDF), 0o644); err != nil {
		t.Fatalf("failed to write test PDF: %v", err)
	}
	outsidePDF := filepath.Join(outsideDir, "secret.pdf")
	if err := os.WriteFile(outsidePDF, []byte(sampleMinimalPDF), 0o644); err != nil {
		t.Fatalf("failed to write outside PDF: %v", err)
	}

	sb, err := sandbox.New([]string{allowedDir}, true)
	if err != nil {
		t.Fatalf("sandbox.New failed: %v", err)
	}

	eng := pdf.NewEngine()
	defer eng.Close()

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "pdf-blick-test",
		Version: "1.0.0",
	}, nil)
	RegisterAll(srv, NewHandler(sb, eng))

	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("srv.Connect failed: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}, nil)
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect failed: %v", err)
	}
	defer clientSession.Close()

	// 1. Test list_pages via MCP client
	listRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_pages",
		Arguments: map[string]any{"path": pdfPath},
	})
	if err != nil {
		t.Fatalf("CallTool(list_pages) failed: %v", err)
	}
	if listRes.IsError {
		t.Fatalf("CallTool(list_pages) returned IsError=true: %+v", listRes.Content)
	}

	// 2. Test get_page (full page) via MCP client
	getRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_page",
		Arguments: map[string]any{"path": "doc.pdf", "page": 1},
	})
	if err != nil {
		t.Fatalf("CallTool(get_page) failed: %v", err)
	}
	if getRes.IsError {
		t.Fatalf("CallTool(get_page) returned IsError=true: %+v", getRes.Content)
	}
	if len(getRes.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(getRes.Content))
	}
	imgContent, ok := getRes.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("expected *mcp.ImageContent, got %T", getRes.Content[0])
	}
	if imgContent.MIMEType != "image/png" {
		t.Fatalf("expected MIMEType image/png, got %s", imgContent.MIMEType)
	}
	decodedImg, err := png.Decode(bytes.NewReader(imgContent.Data))
	if err != nil {
		t.Fatalf("failed to decode returned PNG: %v", err)
	}
	if decodedImg.Bounds().Dx() < 1273 || decodedImg.Bounds().Dx() > 1277 || decodedImg.Bounds().Dy() < 1648 || decodedImg.Bounds().Dy() > 1653 {
		t.Fatalf("unexpected image dimensions at 150 DPI for US Letter: %v", decodedImg.Bounds())
	}

	// 2b. Test get_page with 90 degree rotation via MCP client
	rotRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_page",
		Arguments: map[string]any{"path": "doc.pdf", "page": 1, "rotate": 90},
	})
	if err != nil || rotRes.IsError {
		t.Fatalf("CallTool(get_page rotate 90) failed: err=%v, isError=%v", err, rotRes.IsError)
	}
	rotImgContent := rotRes.Content[0].(*mcp.ImageContent)
	rotImg, err := png.Decode(bytes.NewReader(rotImgContent.Data))
	if err != nil {
		t.Fatalf("failed to decode rotated PNG: %v", err)
	}
	// For US Letter at 150 DPI, normal is ~1275x1650, rotated 90 is ~1650x1275
	if rotImg.Bounds().Dx() < 1648 || rotImg.Bounds().Dx() > 1653 || rotImg.Bounds().Dy() < 1273 || rotImg.Bounds().Dy() > 1277 {
		t.Fatalf("expected rotated 90 deg dimensions ~1650x1275, got %v", rotImg.Bounds())
	}

	// 2c. Test get_page with 180 degree rotation via MCP client
	rot180Res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_page",
		Arguments: map[string]any{"path": "doc.pdf", "page": 1, "rotate": 180},
	})
	if err != nil || rot180Res.IsError {
		t.Fatalf("CallTool(get_page rotation 180) failed: err=%v, isError=%v", err, rot180Res.IsError)
	}
	rot180ImgContent := rot180Res.Content[0].(*mcp.ImageContent)
	rot180Img, err := png.Decode(bytes.NewReader(rot180ImgContent.Data))
	if err != nil {
		t.Fatalf("failed to decode 180 deg PNG: %v", err)
	}
	if rot180Img.Bounds().Dx() < 1273 || rot180Img.Bounds().Dx() > 1277 || rot180Img.Bounds().Dy() < 1648 || rot180Img.Bounds().Dy() > 1653 {
		t.Fatalf("expected 180 deg dimensions ~1275x1650, got %v", rot180Img.Bounds())
	}

	// 3. Test get_page with bounding box zoom (0.25 x 0.25 -> auto-scales to 600 DPI -> same 1275x1650 dimensions!)
	zoomRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_page",
		Arguments: map[string]any{
			"path":   pdfPath,
			"page":   1,
			"left":   0.25,
			"top":    0.25,
			"right":  0.50,
			"bottom": 0.50,
		},
	})
	if err != nil || zoomRes.IsError {
		t.Fatalf("CallTool(get_page zoom) failed: err=%v, isError=%v", err, zoomRes.IsError)
	}
	zoomImgContent := zoomRes.Content[0].(*mcp.ImageContent)
	zoomImg, err := png.Decode(bytes.NewReader(zoomImgContent.Data))
	if err != nil {
		t.Fatalf("failed to decode zoomed PNG: %v", err)
	}
	if zoomImg.Bounds().Dx() < 1273 || zoomImg.Bounds().Dx() > 1277 || zoomImg.Bounds().Dy() < 1648 || zoomImg.Bounds().Dy() > 1653 {
		t.Fatalf("expected zoomed 25%%x25%% crop at auto-600 DPI to be ~1275x1651, got %v", zoomImg.Bounds())
	}

	// 4. Test sandbox rejection on file outside allowed root
	deniedRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_pages",
		Arguments: map[string]any{"path": outsidePDF},
	})
	if err == nil && !deniedRes.IsError {
		t.Fatal("expected sandbox error when accessing outside PDF, but call succeeded")
	}
}
