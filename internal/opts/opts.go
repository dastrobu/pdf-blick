package opts

import (
	"fmt"
	"os"

	"github.com/dastrobu/pdf-blick/internal/pdf"
	"github.com/dastrobu/pdf-blick/internal/tools"
	"github.com/jessevdk/go-flags"
	"github.com/joho/godotenv"
)

// Options defines the top-level command-line options for pdf-blick.
type Options struct {
	Version bool `long:"version" short:"v" description:"Show version information and exit"`

	Run        RunCmd        `command:"run" description:"Run the MCP server over stdio"`
	Completion CompletionCmd `command:"completion" description:"Generate shell completion scripts"`
	Tool       ToolCmd       `command:"tool" description:"Execute a tool directly from the CLI"`
}

// SandboxOpts defines options for configuring file access sandboxing.
type SandboxOpts struct {
	Roots     []string `long:"root" short:"r" env:"PDF_BLICK_ROOT" env-delim:":" description:"Allowed root directory or PDF file path (repeatable)"`
	NoWorkdir bool     `long:"no-workdir" env:"PDF_BLICK_NO_WORKDIR" description:"Disallow access to the current working directory; only allow explicit --root paths"`
}

// EngineOpts defines configurable rendering and metadata limits for the PDF engine.
type EngineOpts struct {
	DefaultDPI        int `long:"default-dpi" env:"PDF_BLICK_DEFAULT_DPI" default:"150" description:"Default rendering resolution in DPI when not specified"`
	MinDPI            int `long:"min-dpi" env:"PDF_BLICK_MIN_DPI" default:"36" description:"Minimum allowed rendering resolution in DPI"`
	MaxDPI            int `long:"max-dpi" env:"PDF_BLICK_MAX_DPI" default:"600" description:"Maximum allowed rendering resolution in DPI (and cap for auto-zoom)"`
	MaxImageDimension int `long:"max-image-dimension" env:"PDF_BLICK_MAX_IMAGE_DIMENSION" default:"4096" description:"Maximum width or height in pixels for a rendered image"`
	DefaultMaxPages   int `long:"default-max-pages" env:"PDF_BLICK_DEFAULT_MAX_PAGES" default:"100" description:"Default maximum number of pages returned by list_pages when end_page is omitted"`
	MaxPagesWindow    int `long:"max-pages-window" env:"PDF_BLICK_MAX_PAGES_WINDOW" default:"200" description:"Maximum number of pages that can be inspected in a single list_pages call"`
	MaxBookmarks      int `long:"max-bookmarks" env:"PDF_BLICK_MAX_BOOKMARKS" default:"200" description:"Maximum number of bookmark nodes returned by list_pages"`
	MaxAttachments    int `long:"max-attachments" env:"PDF_BLICK_MAX_ATTACHMENTS" default:"50" description:"Maximum number of attachment names returned by list_pages"`
	MaxMetadataLen    int `long:"max-metadata-len" env:"PDF_BLICK_MAX_METADATA_LEN" default:"512" description:"Maximum character length of a metadata tag value before truncation"`
}

// ToPDFConfig converts EngineOpts into a pdf.Config.
func (o EngineOpts) ToPDFConfig() pdf.Config {
	return pdf.Config{
		DefaultDPI:          o.DefaultDPI,
		MinDPI:              o.MinDPI,
		MaxDPI:              o.MaxDPI,
		MaxImageDimension:   o.MaxImageDimension,
		DefaultMaxListPages: o.DefaultMaxPages,
		MaxListPagesWindow:  o.MaxPagesWindow,
		MaxBookmarks:        o.MaxBookmarks,
		MaxAttachments:      o.MaxAttachments,
		MaxMetadataValueLen: o.MaxMetadataLen,
	}
}

// RunCmd defines the 'run' command to start the MCP server over stdio.
type RunCmd struct {
	SandboxOpts
	EngineOpts
	Debug   bool `long:"debug" env:"PDF_BLICK_DEBUG" description:"Enable debug logging of MCP requests and responses to stderr"`
	Handler func() error
}

// Execute runs the run command handler.
func (c *RunCmd) Execute(args []string) error {
	if c.Handler != nil {
		return c.Handler()
	}
	return nil
}

// CompletionCmd holds shell completion subcommands.
type CompletionCmd struct {
	Bash CompletionBashCmd `command:"bash" description:"Generate bash completion script"`
}

// CompletionBashCmd represents the 'completion bash' command.
type CompletionBashCmd struct {
	Handler func() error
}

// Execute runs the completion bash command handler.
func (c *CompletionBashCmd) Execute(args []string) error {
	if c.Handler != nil {
		return c.Handler()
	}
	return nil
}

// ToolCmd holds direct CLI tool invocation subcommands.
type ToolCmd struct {
	ListPages ListPagesCmd `command:"list_pages" description:"Inspect PDF metadata, bookmarks, and page dimensions"`
	GetPage   GetPageCmd   `command:"get_page" description:"Render a PDF page or bounding box as a PNG image"`
}

// ListPagesCmd represents the 'tool list_pages' CLI command.
type ListPagesCmd struct {
	SandboxOpts
	EngineOpts
	tools.ListPagesInput
	Handler func(SandboxOpts, EngineOpts, tools.ListPagesInput) error
}

// Execute runs the list_pages tool command handler.
func (c *ListPagesCmd) Execute(args []string) error {
	if c.Handler != nil {
		return c.Handler(c.SandboxOpts, c.EngineOpts, c.ListPagesInput)
	}
	return nil
}

// GetPageCmd represents the 'tool get_page' CLI command.
type GetPageCmd struct {
	SandboxOpts
	EngineOpts
	tools.GetPageInput
	Output  string `long:"output" short:"o" description:"Optional file path to write the rendered PNG image directly instead of printing JSON to stdout"`
	Handler func(SandboxOpts, EngineOpts, tools.GetPageInput, string) error
}

// Execute runs the get_page tool command handler.
func (c *GetPageCmd) Execute(args []string) error {
	if c.Handler != nil {
		return c.Handler(c.SandboxOpts, c.EngineOpts, c.GetPageInput, c.Output)
	}
	return nil
}

// GlobalOpts holds the parsed global options.
var GlobalOpts = Options{}

// Parse parses command-line arguments and environment variables into GlobalOpts.
// It also loads a .env file if present.
func Parse() (*flags.Parser, error) {
	_ = godotenv.Load()

	parser := flags.NewParser(&GlobalOpts, flags.HelpFlag|flags.PassDoubleDash)

	_, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			switch flagsErr.Type {
			case flags.ErrHelp:
				parser.WriteHelp(os.Stdout)
				os.Exit(0)
			case flags.ErrCommandRequired:
				return parser, nil
			default:
				return nil, fmt.Errorf("failed to parse options: %w", err)
			}
		}
		return nil, fmt.Errorf("failed to parse options: %w", err)
	}

	return parser, nil
}

// ParseArgs parses the provided arguments into a fresh Options instance.
// Useful for unit testing flag combinations without mutating GlobalOpts.
func ParseArgs(args []string) (*Options, *flags.Parser, error) {
	var options Options
	parser := flags.NewParser(&options, flags.HelpFlag|flags.PassDoubleDash)
	_, err := parser.ParseArgs(args)
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrCommandRequired {
			return &options, parser, nil
		}
		return nil, nil, fmt.Errorf("failed to parse options: %w", err)
	}
	return &options, parser, nil
}
