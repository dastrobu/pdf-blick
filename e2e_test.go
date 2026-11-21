package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dastrobu/pdf-mcp/internal/opts"
	"github.com/dastrobu/pdf-mcp/internal/pdf"
	"github.com/dastrobu/pdf-mcp/internal/sandbox"
	"github.com/dastrobu/pdf-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestE2E_A4Letter_MCP(t *testing.T) {
	testPDF := filepath.Join("testdata", "A4 Letter.pdf")
	if _, err := os.Stat(testPDF); err != nil {
		t.Fatalf("test PDF fixture not found: %v", err)
	}

	sb, err := sandbox.New(nil, false) // Default workdir enabled
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	eng := pdf.NewEngine()
	defer eng.Close()

	srv := createServer(sb, eng, false)

	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()

	serverSession, err := srv.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("srv.Connect failed: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "e2e-client",
		Version: "1.0.0",
	}, nil)

	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect failed: %v", err)
	}
	defer clientSession.Close()

	// 1. Test list_pages
	listRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_pages",
		Arguments: map[string]any{"path": testPDF},
	})
	if err != nil {
		t.Fatalf("CallTool(list_pages) failed: %v", err)
	}
	if listRes.IsError {
		t.Fatalf("CallTool(list_pages) returned IsError=true: %+v", listRes.Content)
	}
	if len(listRes.Content) != 1 {
		t.Fatalf("expected 1 content item from list_pages, got %d", len(listRes.Content))
	}
	textContent, ok := listRes.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", listRes.Content[0])
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("failed to parse list_pages JSON: %v", err)
	}

	if data["page_count"] != float64(1) {
		t.Errorf("expected page_count 1, got %v", data["page_count"])
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

	// 2. Test get_page (full page)
	getRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_page",
		Arguments: map[string]any{"path": testPDF, "page": 1},
	})
	if err != nil {
		t.Fatalf("CallTool(get_page) failed: %v", err)
	}
	if getRes.IsError {
		t.Fatalf("CallTool(get_page) returned IsError=true: %+v", getRes.Content)
	}
	if len(getRes.Content) != 1 {
		t.Fatalf("expected 1 content item from get_page, got %d", len(getRes.Content))
	}
	imgContent, ok := getRes.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("expected ImageContent, got %T", getRes.Content[0])
	}
	if imgContent.MIMEType != "image/png" {
		t.Errorf("expected MIMEType image/png, got %s", imgContent.MIMEType)
	}

	img, _, err := image.Decode(bytes.NewReader(imgContent.Data))
	if err != nil {
		t.Fatalf("failed to decode rendered PNG: %v", err)
	}
	bounds := img.Bounds()
	// At 150 DPI, A4 portrait is 1241 x 1754 px
	if bounds.Dx() != 1241 || bounds.Dy() != 1754 {
		t.Errorf("expected 1241x1754 image, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// 3. Test get_page with crop bounding box
	left, top, right, bottom := 0.1, 0.1, 0.6, 0.6
	cropRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_page",
		Arguments: map[string]any{
			"path":   testPDF,
			"page":   1,
			"left":   left,
			"top":    top,
			"right":  right,
			"bottom": bottom,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(get_page, crop) failed: %v", err)
	}
	if cropRes.IsError {
		t.Fatalf("CallTool(get_page, crop) returned IsError=true: %+v", cropRes.Content)
	}
	cropImgContent, ok := cropRes.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("expected ImageContent, got %T", cropRes.Content[0])
	}
	cropImg, _, err := image.Decode(bytes.NewReader(cropImgContent.Data))
	if err != nil {
		t.Fatalf("failed to decode crop PNG: %v", err)
	}
	cropBounds := cropImg.Bounds()
	if cropBounds.Dx() <= 0 || cropBounds.Dy() <= 0 {
		t.Errorf("invalid crop bounds: %v", cropBounds)
	}
}

func TestE2E_A4Letter_DirectCLIHandlers(t *testing.T) {
	registerToolHandlers()

	testPDF := filepath.Join("testdata", "A4 Letter.pdf")
	if _, err := os.Stat(testPDF); err != nil {
		t.Fatalf("test PDF fixture not found: %v", err)
	}

	// Test ListPages CLI handler
	err := opts.GlobalOpts.Tool.ListPages.Handler(
		opts.SandboxOpts{},
		opts.EngineOpts{},
		tools.ListPagesInput{Path: testPDF},
	)
	if err != nil {
		t.Fatalf("ListPages CLI handler failed: %v", err)
	}

	// Test GetPage CLI handler writing to output file
	outPath := filepath.Join(t.TempDir(), "rendered.png")
	err = opts.GlobalOpts.Tool.GetPage.Handler(
		opts.SandboxOpts{},
		opts.EngineOpts{},
		tools.GetPageInput{Path: testPDF, Page: 1},
		outPath,
	)
	if err != nil {
		t.Fatalf("GetPage CLI handler failed: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("rendered output file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("rendered output file is empty")
	}
}
