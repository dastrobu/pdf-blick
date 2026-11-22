package tools

import (
	"context"
	"fmt"

	"github.com/dastrobu/pdf-mcp/internal/pdf"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetPageInput defines parameters for the get_page tool.
type GetPageInput struct {
	Path   string   `json:"path" long:"path" required:"true" description:"Path to the PDF file" jsonschema:"Path to the PDF file"`
	Page   int      `json:"page" long:"page" required:"true" description:"1-indexed page number to render" jsonschema:"1-indexed page number to render"`
	DPI    *int     `json:"dpi,omitempty" long:"dpi" description:"Resolution in DPI (36-600, default: 150, auto-scaled when cropping)" jsonschema:"Optional rendering resolution in DPI (36-600). Defaults to 150 for full page, or automatically scales up to 600 DPI when zooming into a bounding box."`
	Rotate *int     `json:"rotate,omitempty" long:"rotate" description:"Optional page rotation in degrees clockwise (0, 90, 180, 270, default: 0, applied before cropping)" jsonschema:"Optional page rotation in degrees clockwise (0, 90, 180, 270). Applied before cropping so bounding box coordinates apply to the rotated page. Defaults to 0."`
	Left   *float64 `json:"left,omitempty" long:"left" description:"Left edge of bounding box (0.0 to 1.0, default: 0.0)" jsonschema:"Optional left edge of normalized crop bounding box (0.0 to 1.0, applied after rotation with origin at top-left of the rotated page, default 0.0)"`
	Top    *float64 `json:"top,omitempty" long:"top" description:"Top edge of bounding box (0.0 to 1.0, default: 0.0)" jsonschema:"Optional top edge of normalized crop bounding box (0.0 to 1.0, applied after rotation with origin at top-left of the rotated page, default 0.0)"`
	Right  *float64 `json:"right,omitempty" long:"right" description:"Right edge of bounding box (0.0 to 1.0, default: 1.0)" jsonschema:"Optional right edge of normalized crop bounding box (0.0 to 1.0, applied after rotation with origin at top-left of the rotated page, default 1.0)"`
	Bottom *float64 `json:"bottom,omitempty" long:"bottom" description:"Bottom edge of bounding box (0.0 to 1.0, default: 1.0)" jsonschema:"Optional bottom edge of normalized crop bounding box (0.0 to 1.0, applied after rotation with origin at top-left of the rotated page, default 1.0)"`
}

// RegisterGetPage registers the get_page tool with the MCP server.
func (h *Handler) RegisterGetPage(srv *mcp.Server) {
	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "get_page",
			Description: "Renders a PDF page or normalized crop region (bounding box) to a PNG image.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "Get PDF Page",
				ReadOnlyHint:    true,
				IdempotentHint:  true,
				DestructiveHint: new(false),
				OpenWorldHint:   new(false),
			},
		},
		h.HandleGetPage,
	)
}

// HandleGetPage validates the input path against the sandbox and renders the requested page or bounding box.
func (h *Handler) HandleGetPage(ctx context.Context, req *mcp.CallToolRequest, input GetPageInput) (*mcp.CallToolResult, any, error) {
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

	res, err := h.Engine.RenderPage(ctx, pdf.RenderOptions{
		Path:   canonicalPath,
		Page:   input.Page,
		DPI:    input.DPI,
		Rotate: input.Rotate,
		Left:   input.Left,
		Top:    input.Top,
		Right:  input.Right,
		Bottom: input.Bottom,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to render page: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{
				Data:     res.PNGBytes,
				MIMEType: "image/png",
			},
		},
	}, nil, nil
}
