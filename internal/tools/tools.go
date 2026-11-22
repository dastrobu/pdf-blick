package tools

import (
	"github.com/dastrobu/pdf-blick/internal/pdf"
	"github.com/dastrobu/pdf-blick/internal/sandbox"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Handler holds dependencies for MCP tool executions.
type Handler struct {
	Sandbox *sandbox.Sandbox
	Engine  *pdf.Engine
}

// NewHandler creates a new tools Handler.
func NewHandler(sb *sandbox.Sandbox, eng *pdf.Engine) *Handler {
	return &Handler{
		Sandbox: sb,
		Engine:  eng,
	}
}

// RegisterAll registers all pdf-blick tools with the given MCP server.
func RegisterAll(srv *mcp.Server, h *Handler) {
	h.RegisterListPages(srv)
	h.RegisterGetPage(srv)
}
