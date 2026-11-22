package main

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/dastrobu/pdf-blick/internal/completion"
	applog "github.com/dastrobu/pdf-blick/internal/log"
	"github.com/dastrobu/pdf-blick/internal/opts"
	"github.com/dastrobu/pdf-blick/internal/pdf"
	"github.com/dastrobu/pdf-blick/internal/sandbox"
	"github.com/dastrobu/pdf-blick/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	// Version information (set by GoReleaser via ldflags)
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	serverName = "pdf-blick"
)

func main() {
	// Set up command handlers before parsing
	opts.GlobalOpts.Completion.Bash.Handler = func() error {
		completion.GenerateBash()
		return nil
	}
	opts.GlobalOpts.Run.Handler = func() error {
		return run(&opts.GlobalOpts.Run)
	}

	registerToolHandlers()

	// Parse command-line options
	parser, err := opts.Parse()
	if err != nil {
		log.Fatalf("%v", err)
	}

	// Handle version flag
	if opts.GlobalOpts.Version {
		fmt.Printf("pdf-blick version %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
		return
	}

	// Check if a command was executed
	if parser.Command.Active != nil {
		return
	}

	// No command specified - show help
	parser.WriteHelp(os.Stdout)
}

// setupLogger creates and adds the appropriate logger to the context
func setupLogger(ctx context.Context, debug bool) context.Context {
	if debug {
		return applog.WithLogger(ctx, log.Default())
	}
	return applog.WithLogger(ctx, log.New(io.Discard, "", 0))
}

// debugMiddleware logs all MCP requests and responses when debug is enabled
func debugMiddleware(debug bool) func(mcp.MethodHandler) mcp.MethodHandler {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			ctx = setupLogger(ctx, debug)

			if req != nil {
				p := req.GetParams()
				j, _ := json.Marshal(p, jsontext.WithIndent("  "))
				log.Printf("[DEBUG] MCP Request: %s\nParams: %s\n", method, string(j))
			} else {
				log.Printf("[DEBUG] MCP Request: %s\n", method)
			}

			result, err := next(ctx, method, req)

			if err != nil {
				log.Printf("[DEBUG] MCP Response: %s\nError: %v\n", method, err)
			} else if result != nil {
				resultJSON, _ := json.Marshal(result, jsontext.WithIndent("  "))
				log.Printf("[DEBUG] MCP Response: %s\nResult: %s\n", method, string(resultJSON))
			} else {
				log.Printf("[DEBUG] MCP Response: %s\n", method)
			}

			return result, err
		}
	}
}

// createServer creates and configures a new MCP server instance
func createServer(sb *sandbox.Sandbox, eng *pdf.Engine, debug bool) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: version,
	}, nil)

	if debug {
		srv.AddReceivingMiddleware(debugMiddleware(debug))
	}

	tools.RegisterAll(srv, tools.NewHandler(sb, eng))

	return srv
}

func run(options *opts.RunCmd) error {
	ctx := context.Background()
	ctx = setupLogger(ctx, options.Debug)

	sb, err := sandbox.New(options.Roots, options.NoWorkdir)
	if err != nil {
		return fmt.Errorf("failed to initialize sandbox: %w", err)
	}

	eng := pdf.NewEngineWithConfig(options.EngineOpts.ToPDFConfig())
	defer eng.Close()
	eng.Warmup()

	log.Printf("pdf-blick v%s (commit: %s, built: %s) initialized\n", version, commit, date)
	log.Printf("Allowed roots: %v (no-workdir: %v)\n", sb.AllowedRoots(), options.NoWorkdir)

	srv := createServer(sb, eng, options.Debug)

	log.Println("Starting MCP server using STDIO transport")
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func registerToolHandlers() {
	opts.GlobalOpts.Tool.ListPages.Handler = func(sbOpts opts.SandboxOpts, engOpts opts.EngineOpts, input tools.ListPagesInput) error {
		sb, err := sandbox.New(sbOpts.Roots, sbOpts.NoWorkdir)
		if err != nil {
			return fmt.Errorf("failed to initialize sandbox: %w", err)
		}
		eng := pdf.NewEngineWithConfig(engOpts.ToPDFConfig())
		defer eng.Close()

		h := tools.NewHandler(sb, eng)
		_, data, err := h.HandleListPages(context.Background(), nil, input)
		if err != nil {
			return err
		}
		if err := json.MarshalWrite(os.Stdout, data, jsontext.WithIndent("  ")); err != nil {
			return err
		}
		fmt.Println()
		return nil
	}

	opts.GlobalOpts.Tool.GetPage.Handler = func(sbOpts opts.SandboxOpts, engOpts opts.EngineOpts, input tools.GetPageInput, outputPath string) error {
		sb, err := sandbox.New(sbOpts.Roots, sbOpts.NoWorkdir)
		if err != nil {
			return fmt.Errorf("failed to initialize sandbox: %w", err)
		}
		eng := pdf.NewEngineWithConfig(engOpts.ToPDFConfig())
		defer eng.Close()

		h := tools.NewHandler(sb, eng)
		res, _, err := h.HandleGetPage(context.Background(), nil, input)
		if err != nil {
			return err
		}

		if outputPath != "" {
			var pngData []byte
			for _, c := range res.Content {
				if img, ok := c.(*mcp.ImageContent); ok {
					pngData = img.Data
					break
				}
			}
			if len(pngData) == 0 {
				return fmt.Errorf("no image content returned")
			}
			if err := os.WriteFile(outputPath, pngData, 0644); err != nil {
				return fmt.Errorf("failed to write image to %s: %w", outputPath, err)
			}
			fmt.Fprintf(os.Stderr, "Rendered page written to %s (%d bytes)\n", outputPath, len(pngData))
			return nil
		}

		if err := json.MarshalWrite(os.Stdout, res, jsontext.WithIndent("  ")); err != nil {
			return err
		}
		fmt.Println()
		return nil
	}
}
