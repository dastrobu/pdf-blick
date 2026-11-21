package tools

import (
	"context"
	"fmt"

	"github.com/dastrobu/pdf-mcp/internal/pdf"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListPagesInput defines parameters for the list_pages tool.
type ListPagesInput struct {
	Path      string `json:"path" long:"path" required:"true" description:"Path to the PDF file" jsonschema:"Path to the PDF file to inspect"`
	StartPage *int   `json:"start_page,omitempty" long:"start-page" description:"Optional 1-indexed start page for page metadata pagination (default: 1)" jsonschema:"Optional 1-indexed start page for paginating per-page metadata on large PDFs (defaults to 1)"`
	EndPage   *int   `json:"end_page,omitempty" long:"end-page" description:"Optional 1-indexed end page for page metadata pagination (default: start_page + 99, max window 200)" jsonschema:"Optional 1-indexed end page for paginating per-page metadata on large PDFs (defaults to start_page + 99, max 200 pages per call)"`
}

// RegisterListPages registers the list_pages tool with the MCP server.
func (h *Handler) RegisterListPages(srv *mcp.Server) {
	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "list_pages",
			Description: "Inspects a PDF document to retrieve total page count, bookmarks/outlines, and per-page dimensions and rotations.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "List PDF Pages",
				ReadOnlyHint:    true,
				IdempotentHint:  true,
				DestructiveHint: new(false),
				OpenWorldHint:   new(false),
			},
		},
		h.HandleListPages,
	)
}

// HandleListPages validates the path against the sandbox and retrieves page metadata using the engine.
func (h *Handler) HandleListPages(ctx context.Context, req *mcp.CallToolRequest, input ListPagesInput) (*mcp.CallToolResult, any, error) {
	if h.Sandbox == nil {
		return nil, nil, fmt.Errorf("sandbox not initialized")
	}
	canonicalPath, err := h.Sandbox.ValidatePath(input.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("path validation failed: %w", err)
	}

	if h.Engine == nil {
		return nil, nil, fmt.Errorf("pdf engine not initialized")
	}

	data, err := h.Engine.ListPages(ctx, canonicalPath, pdf.ListPagesOptions{
		StartPage: input.StartPage,
		EndPage:   input.EndPage,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list pages: %w", err)
	}

	return nil, data, nil
}
