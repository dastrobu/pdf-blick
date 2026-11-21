package pdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

const (
	DefaultDPI                 = 150
	DefaultMinDPI              = 36
	DefaultMaxDPI              = 600
	DefaultMaxImageDimension   = 4096
	DefaultMaxListPages        = 100
	DefaultMaxListPagesWindow  = 200
	DefaultMaxBookmarks        = 200
	DefaultMaxAttachments      = 50
	DefaultMaxMetadataValueLen = 512

	// Backwards-compatible aliases
	MaxListPagesWindow = DefaultMaxListPagesWindow
	MaxBookmarks       = DefaultMaxBookmarks
	MaxAttachments     = DefaultMaxAttachments

	DefaultInstanceTimeout = 30 * time.Second
	PointsPerInch          = 72.0
	OpaqueWhiteColor       = 0xFFFFFFFF
	PageFormatTolerancePt  = 3.0
	AspectRatioTolerance   = 0.05
)

// Config holds runtime configuration options for the PDF engine.
type Config struct {
	DefaultDPI          int
	MinDPI              int
	MaxDPI              int
	MaxImageDimension   int
	DefaultMaxListPages int
	MaxListPagesWindow  int
	MaxBookmarks        int
	MaxAttachments      int
	MaxMetadataValueLen int
}

// DefaultConfig returns the standard default engine configuration.
func DefaultConfig() Config {
	return Config{
		DefaultDPI:          DefaultDPI,
		MinDPI:              DefaultMinDPI,
		MaxDPI:              DefaultMaxDPI,
		MaxImageDimension:   DefaultMaxImageDimension,
		DefaultMaxListPages: DefaultMaxListPages,
		MaxListPagesWindow:  DefaultMaxListPagesWindow,
		MaxBookmarks:        DefaultMaxBookmarks,
		MaxAttachments:      DefaultMaxAttachments,
		MaxMetadataValueLen: DefaultMaxMetadataValueLen,
	}
}

// Engine manages the PDFium WebAssembly runtime and document processing.
type Engine struct {
	cfg              Config
	mu               sync.Mutex
	initOnce         sync.Once
	initErr          error
	pool             pdfium.Pool
	instance         pdfium.Pdfium
	compilationCache wazero.CompilationCache
	closed           bool
}

// ListPagesOptions specifies pagination options for ListPages.
type ListPagesOptions struct {
	StartPage *int // 1-indexed start page (optional, nil defaults to 1)
	EndPage   *int // 1-indexed end page (optional, nil defaults to min(pageCount, startPage+99))
}

// RenderOptions specifies parameters for rendering a PDF page or region.
type RenderOptions struct {
	Path   string
	Page   int      // 1-indexed page number (1..page_count)
	DPI    *int     // optional; nil means auto (150 base DPI, auto-scaled when cropped)
	Left   *float64 // optional 0.0..1.0 (default 0.0)
	Top    *float64 // optional 0.0..1.0 (default 0.0)
	Right  *float64 // optional 0.0..1.0 (default 1.0)
	Bottom *float64 // optional 0.0..1.0 (default 1.0)
}

// RenderResult holds rendered image data and metadata.
type RenderResult struct {
	PNGBytes     []byte
	WidthPx      int
	HeightPx     int
	EffectiveDPI int
	Page         int
	TotalPages   int
	Left         float64
	Top          float64
	Right        float64
	Bottom       float64
}

// NewEngine creates a new uninitialized Engine with DefaultConfig().
func NewEngine() *Engine {
	return NewEngineWithConfig(DefaultConfig())
}

// NewEngineWithConfig creates a new uninitialized Engine with the provided configuration.
func NewEngineWithConfig(cfg Config) *Engine {
	if cfg.DefaultDPI <= 0 {
		cfg.DefaultDPI = DefaultDPI
	}
	if cfg.MinDPI <= 0 {
		cfg.MinDPI = DefaultMinDPI
	}
	if cfg.MaxDPI <= 0 {
		cfg.MaxDPI = DefaultMaxDPI
	}
	if cfg.MaxImageDimension <= 0 {
		cfg.MaxImageDimension = DefaultMaxImageDimension
	}
	if cfg.DefaultMaxListPages <= 0 {
		cfg.DefaultMaxListPages = DefaultMaxListPages
	}
	if cfg.MaxListPagesWindow <= 0 {
		cfg.MaxListPagesWindow = DefaultMaxListPagesWindow
	}
	if cfg.MaxBookmarks <= 0 {
		cfg.MaxBookmarks = DefaultMaxBookmarks
	}
	if cfg.MaxAttachments <= 0 {
		cfg.MaxAttachments = DefaultMaxAttachments
	}
	if cfg.MaxMetadataValueLen <= 0 {
		cfg.MaxMetadataValueLen = DefaultMaxMetadataValueLen
	}

	if cfg.MinDPI > cfg.MaxDPI {
		cfg.MinDPI, cfg.MaxDPI = cfg.MaxDPI, cfg.MinDPI
	}
	if cfg.DefaultDPI < cfg.MinDPI {
		cfg.DefaultDPI = cfg.MinDPI
	}
	if cfg.DefaultDPI > cfg.MaxDPI {
		cfg.DefaultDPI = cfg.MaxDPI
	}

	return &Engine{
		cfg: cfg,
	}
}

// Config returns a copy of the engine's active configuration.
func (e *Engine) Config() Config {
	return e.cfg
}

// Warmup initializes the WebAssembly engine in the background.
func (e *Engine) Warmup() {
	go func() {
		_ = e.ensureInit()
	}()
}

func (e *Engine) ensureInit() error {
	e.initOnce.Do(func() {
		cacheBase, err := os.UserCacheDir()
		if err != nil || cacheBase == "" {
			cacheBase = os.TempDir()
		}
		cacheDir := filepath.Join(cacheBase, "pdf-mcp", "wazero")
		_ = os.MkdirAll(cacheDir, 0o755)

		runtimeCfg := wazero.NewRuntimeConfig().WithCoreFeatures(
			api.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling,
		)
		cache, err := wazero.NewCompilationCacheWithDir(cacheDir)
		if err == nil {
			runtimeCfg = runtimeCfg.WithCompilationCache(cache)
		}

		pool, err := webassembly.Init(webassembly.Config{
			MinIdle:       1,
			MaxIdle:       1,
			MaxTotal:      1,
			RuntimeConfig: runtimeCfg,
		})
		if err != nil {
			if cache != nil {
				_ = cache.Close(context.Background())
			}
			e.initErr = fmt.Errorf("failed to initialize PDFium webassembly pool: %w", err)
			return
		}

		instance, err := pool.GetInstance(DefaultInstanceTimeout)
		if err != nil {
			_ = pool.Close()
			if cache != nil {
				_ = cache.Close(context.Background())
			}
			e.initErr = fmt.Errorf("failed to get PDFium instance: %w", err)
			return
		}

		e.mu.Lock()
		defer e.mu.Unlock()
		if e.closed {
			_ = instance.Close()
			_ = pool.Close()
			if cache != nil {
				_ = cache.Close(context.Background())
			}
			e.initErr = errors.New("engine is closed")
			return
		}

		e.compilationCache = cache
		e.pool = pool
		e.instance = instance
	})
	return e.initErr
}

// Close terminates the engine, returning resources to the system.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true

	var errs []error
	if e.instance != nil {
		if err := e.instance.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close PDFium instance: %w", err))
		}
		e.instance = nil
	}
	if e.pool != nil {
		if err := e.pool.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close PDFium pool: %w", err))
		}
		e.pool = nil
	}
	if e.compilationCache != nil {
		if err := e.compilationCache.Close(context.Background()); err != nil {
			errs = append(errs, fmt.Errorf("failed to close compilation cache: %w", err))
		}
		e.compilationCache = nil
	}

	return errors.Join(errs...)
}

// ListPages inspects the PDF document and returns metadata for the document and pages.
func (e *Engine) ListPages(ctx context.Context, pdfPath string, opts ...ListPagesOptions) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var opt ListPagesOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	if err := e.ensureInit(); err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil, errors.New("engine is closed")
	}

	f, err := os.Open(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", pdfPath, err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %q: %w", pdfPath, err)
	}

	doc, err := e.instance.OpenDocument(&requests.OpenDocument{
		FileReader:     f,
		FileReaderSize: st.Size(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF document: %w", err)
	}
	defer e.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	pcResp, err := e.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}
	pageCount := pcResp.PageCount

	startPage := 1
	endPage := 0
	if pageCount > 0 {
		if opt.StartPage != nil {
			if *opt.StartPage < 1 {
				return nil, errors.New("start_page must be >= 1")
			}
			startPage = *opt.StartPage
		}
		if startPage > pageCount {
			return nil, fmt.Errorf("start_page %d out of bounds (document has %d pages)", startPage, pageCount)
		}

		if opt.EndPage != nil {
			if *opt.EndPage < 1 {
				return nil, errors.New("end_page must be >= 1")
			}
			if *opt.EndPage < startPage {
				return nil, fmt.Errorf("end_page %d must be >= start_page %d", *opt.EndPage, startPage)
			}
			endPage = min(*opt.EndPage, pageCount)
			if endPage-startPage+1 > e.cfg.MaxListPagesWindow {
				endPage = startPage + e.cfg.MaxListPagesWindow - 1
			}
		} else {
			endPage = min(pageCount, startPage+e.cfg.DefaultMaxListPages-1)
		}
	}

	var pdfVersion string
	if v, err := e.instance.FPDF_GetFileVersion(&requests.FPDF_GetFileVersion{Document: doc.Document}); err == nil && v != nil && v.FileVersion > 0 {
		pdfVersion = fmt.Sprintf("%.1f", float64(v.FileVersion)/10.0)
	}

	var isTagged bool
	if t, err := e.instance.FPDFCatalog_IsTagged(&requests.FPDFCatalog_IsTagged{Document: doc.Document}); err == nil && t != nil {
		isTagged = t.IsTagged
	}

	var hasValidXref bool
	if x, err := e.instance.FPDF_DocumentHasValidCrossReferenceTable(&requests.FPDF_DocumentHasValidCrossReferenceTable{Document: doc.Document}); err == nil && x != nil {
		hasValidXref = x.DocumentHasValidCrossReferenceTable
	}

	formType := "none"
	if ft, err := e.instance.FPDF_GetFormType(&requests.FPDF_GetFormType{Document: doc.Document}); err == nil && ft != nil {
		switch ft.FormType {
		case 1:
			formType = "acroform"
		case 2:
			formType = "xfa_full"
		case 3:
			formType = "xfa_foreground"
		default:
			formType = "none"
		}
	}

	pageMode := "use_none"
	if pm, err := e.instance.FPDFDoc_GetPageMode(&requests.FPDFDoc_GetPageMode{Document: doc.Document}); err == nil && pm != nil {
		switch int(pm.PageMode) {
		case 1:
			pageMode = "use_outlines"
		case 2:
			pageMode = "use_thumbs"
		case 3:
			pageMode = "fullscreen"
		case 4:
			pageMode = "use_oc"
		case 5:
			pageMode = "use_attachments"
		default:
			pageMode = "use_none"
		}
	}

	secRev := -1
	if sh, err := e.instance.FPDF_GetSecurityHandlerRevision(&requests.FPDF_GetSecurityHandlerRevision{Document: doc.Document}); err == nil && sh != nil {
		secRev = sh.SecurityHandlerRevision
	}

	permissions := map[string]any{}
	hasRestrictedPerm := false
	if dp, err := e.instance.FPDF_GetDocPermissions(&requests.FPDF_GetDocPermissions{Document: doc.Document}); err == nil && dp != nil {
		perms := map[string]bool{
			"print_document":                           dp.PrintDocument,
			"modify_contents":                          dp.ModifyContents,
			"copy_or_extract_text":                     dp.CopyOrExtractText,
			"add_or_modify_text_annotations":           dp.AddOrModifyTextAnnotations,
			"fill_in_interactive_form_fields":          dp.FillInInteractiveFormFields,
			"create_or_modify_interactive_form_fields": dp.CreateOrModifyInteractiveFormFields,
			"fill_in_existing_interactive_form_fields": dp.FillInExistingInteractiveFormFields,
			"extract_text_and_graphics":                dp.ExtractTextAndGraphics,
			"assemble_document":                        dp.AssembleDocument,
			"print_document_as_faithful_digital_copy":  dp.PrintDocumentAsFaithfulDigitalCopy,
		}
		for k, v := range perms {
			permissions[k] = v
			if !v {
				hasRestrictedPerm = true
			}
		}
	}

	attachmentCount := 0
	var attachments []string
	if ac, err := e.instance.FPDFDoc_GetAttachmentCount(&requests.FPDFDoc_GetAttachmentCount{Document: doc.Document}); err == nil && ac != nil {
		attachmentCount = ac.AttachmentCount
		if attachmentCount > 0 {
			limit := attachmentCount
			if limit > e.cfg.MaxAttachments {
				limit = e.cfg.MaxAttachments
			}
			attachments = make([]string, 0, limit)
			for i := 0; i < limit; i++ {
				att, err := e.instance.FPDFDoc_GetAttachment(&requests.FPDFDoc_GetAttachment{Document: doc.Document, Index: i})
				if err == nil && att != nil {
					nameResp, err := e.instance.FPDFAttachment_GetName(&requests.FPDFAttachment_GetName{Attachment: att.Attachment})
					if err == nil && nameResp != nil {
						attachments = append(attachments, nameResp.Name)
					}
				}
			}
		}
	}

	jsActionCount := 0
	if jsc, err := e.instance.FPDFDoc_GetJavaScriptActionCount(&requests.FPDFDoc_GetJavaScriptActionCount{Document: doc.Document}); err == nil && jsc != nil {
		jsActionCount = jsc.JavaScriptActionCount
	}

	signatureCount := 0
	if sig, err := e.instance.FPDF_GetSignatureCount(&requests.FPDF_GetSignatureCount{Document: doc.Document}); err == nil && sig != nil {
		signatureCount = sig.Count
	}

	metadata := map[string]string{}
	if md, err := e.instance.GetMetaData(&requests.GetMetaData{Document: doc.Document}); err == nil && md != nil {
		for _, tag := range md.Tags {
			if tag.Value != "" {
				val := tag.Value
				runes := []rune(val)
				if len(runes) > e.cfg.MaxMetadataValueLen {
					val = string(runes[:e.cfg.MaxMetadataValueLen]) + "... (truncated)"
				}
				metadata[toSnakeCase(tag.Tag)] = val
			}
		}
	}

	pageTitles := make(map[int]string)
	totalBookmarks := 0
	bookmarksTruncated := false

	var convertBookmark func(b responses.GetBookmarksBookmark) (map[string]any, bool)
	convertBookmark = func(b responses.GetBookmarksBookmark) (map[string]any, bool) {
		var dest *responses.DestInfo
		if b.DestInfo != nil {
			dest = b.DestInfo
		} else if b.ActionInfo != nil && b.ActionInfo.DestInfo != nil {
			dest = b.ActionInfo.DestInfo
		}
		var page int
		if dest != nil {
			page = dest.PageIndex + 1
			if b.Title != "" && page > 0 {
				if existing, ok := pageTitles[page]; ok && existing != "" {
					pageTitles[page] = existing + "; " + b.Title
				} else {
					pageTitles[page] = b.Title
				}
			}
		}

		if totalBookmarks >= e.cfg.MaxBookmarks {
			bookmarksTruncated = true
			for _, ch := range b.Children {
				convertBookmark(ch)
			}
			return nil, false
		}

		totalBookmarks++
		bm := map[string]any{
			"title": b.Title,
		}
		if dest != nil && page > 0 {
			bm["page"] = page
		}

		if len(b.Children) > 0 {
			var children []map[string]any
			for _, ch := range b.Children {
				if childNode, ok := convertBookmark(ch); ok {
					children = append(children, childNode)
				}
			}
			if len(children) > 0 {
				bm["children"] = children
			}
		}
		return bm, true
	}

	bookmarks := make([]map[string]any, 0)
	if bmResp, err := e.instance.GetBookmarks(&requests.GetBookmarks{Document: doc.Document}); err == nil && bmResp != nil {
		for _, b := range bmResp.Bookmarks {
			if node, ok := convertBookmark(b); ok {
				bookmarks = append(bookmarks, node)
			}
		}
	}

	var pages []map[string]any
	var defaultLayout map[string]any

	if pageCount > 0 && startPage <= endPage {
		pages = make([]map[string]any, 0, endPage-startPage+1)
		for i := startPage - 1; i < endPage; i++ {
			pageReq := requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc.Document,
					Index:    i,
				},
			}
			pageNum := i + 1

			var wPt, hPt float64
			if ps, err := e.instance.FPDF_GetPageSizeByIndex(&requests.FPDF_GetPageSizeByIndex{Document: doc.Document, Index: i}); err == nil && ps != nil {
				wPt = ps.Width
				hPt = ps.Height
			}
			wPtRounded := math.Round(wPt*100) / 100
			hPtRounded := math.Round(hPt*100) / 100

			format := detectPageFormat(wPt, hPt)

			var orientation string
			if hPt > wPt {
				orientation = "portrait"
			} else if wPt > hPt {
				orientation = "landscape"
			} else {
				orientation = "square"
			}

			rotDeg := 0
			if rot, err := e.instance.FPDFPage_GetRotation(&requests.FPDFPage_GetRotation{Page: pageReq}); err == nil && rot != nil {
				switch rot.PageRotation {
				case 1:
					rotDeg = 90
				case 2:
					rotDeg = 180
				case 3:
					rotDeg = 270
				default:
					rotDeg = 0
				}
			}

			boxes := map[string]any{}
			var mbLeft, mbBottom, mbRight, mbTop float32
			if mb, err := e.instance.FPDFPage_GetMediaBox(&requests.FPDFPage_GetMediaBox{Page: pageReq}); err == nil && mb != nil && !(mb.Left == 0 && mb.Bottom == 0 && mb.Right == 0 && mb.Top == 0) {
				mbLeft = mb.Left
				mbBottom = mb.Bottom
				mbRight = mb.Right
				mbTop = mb.Top
				boxes["media_box"] = map[string]any{
					"left":   math.Round(float64(mb.Left)*100) / 100,
					"bottom": math.Round(float64(mb.Bottom)*100) / 100,
					"right":  math.Round(float64(mb.Right)*100) / 100,
					"top":    math.Round(float64(mb.Top)*100) / 100,
				}
			} else if bb, err := e.instance.FPDF_GetPageBoundingBox(&requests.FPDF_GetPageBoundingBox{Page: pageReq}); err == nil && bb != nil {
				mbLeft = bb.Rect.Left
				mbBottom = bb.Rect.Bottom
				mbRight = bb.Rect.Right
				mbTop = bb.Rect.Top
				boxes["media_box"] = map[string]any{
					"left":   math.Round(float64(mbLeft)*100) / 100,
					"bottom": math.Round(float64(mbBottom)*100) / 100,
					"right":  math.Round(float64(mbRight)*100) / 100,
					"top":    math.Round(float64(mbTop)*100) / 100,
				}
			}

			addBox := func(name string, left, bottom, right, top float32) {
				isZero := left == 0 && bottom == 0 && right == 0 && top == 0
				isSameAsMedia := left == mbLeft && bottom == mbBottom && right == mbRight && top == mbTop
				if !isZero && !isSameAsMedia {
					boxes[name] = map[string]any{
						"left":   math.Round(float64(left)*100) / 100,
						"bottom": math.Round(float64(bottom)*100) / 100,
						"right":  math.Round(float64(right)*100) / 100,
						"top":    math.Round(float64(top)*100) / 100,
					}
				}
			}

			if cb, err := e.instance.FPDFPage_GetCropBox(&requests.FPDFPage_GetCropBox{Page: pageReq}); err == nil && cb != nil {
				addBox("crop_box", cb.Left, cb.Bottom, cb.Right, cb.Top)
			}
			if blb, err := e.instance.FPDFPage_GetBleedBox(&requests.FPDFPage_GetBleedBox{Page: pageReq}); err == nil && blb != nil {
				addBox("bleed_box", blb.Left, blb.Bottom, blb.Right, blb.Top)
			}
			if tb, err := e.instance.FPDFPage_GetTrimBox(&requests.FPDFPage_GetTrimBox{Page: pageReq}); err == nil && tb != nil {
				addBox("trim_box", tb.Left, tb.Bottom, tb.Right, tb.Top)
			}
			if ab, err := e.instance.FPDFPage_GetArtBox(&requests.FPDFPage_GetArtBox{Page: pageReq}); err == nil && ab != nil {
				addBox("art_box", ab.Left, ab.Bottom, ab.Right, ab.Top)
			}

			if defaultLayout == nil {
				defaultLayout = map[string]any{
					"width_pt":    wPtRounded,
					"height_pt":   hPtRounded,
					"format":      format,
					"orientation": orientation,
					"rotation":    rotDeg,
				}
				if len(boxes) > 0 {
					defaultLayout["boxes"] = boxes
				}
			}

			charCount := 0
			if txt, err := e.instance.GetPageText(&requests.GetPageText{Page: pageReq}); err == nil && txt != nil {
				charCount = len([]rune(strings.TrimSpace(txt.Text)))
			}

			objCount := 0
			if oc, err := e.instance.FPDFPage_CountObjects(&requests.FPDFPage_CountObjects{Page: pageReq}); err == nil && oc != nil {
				objCount = oc.Count
			}

			pageData := map[string]any{
				"page":         pageNum,
				"char_count":   charCount,
				"object_count": objCount,
			}

			if pl, err := e.instance.FPDF_GetPageLabel(&requests.FPDF_GetPageLabel{Document: doc.Document, Page: i}); err == nil && pl != nil && pl.Label != "" {
				pageData["label"] = pl.Label
			}
			if title, ok := pageTitles[pageNum]; ok && title != "" {
				pageData["title"] = title
			}

			if wPtRounded != defaultLayout["width_pt"] || hPtRounded != defaultLayout["height_pt"] {
				pageData["width_pt"] = wPtRounded
				pageData["height_pt"] = hPtRounded
				pageData["format"] = format
				pageData["orientation"] = orientation
			}

			if rotDeg != 0 {
				pageData["rotation"] = rotDeg
			}

			hasTrans := false
			if ht, err := e.instance.FPDFPage_HasTransparency(&requests.FPDFPage_HasTransparency{Page: pageReq}); err == nil && ht != nil {
				hasTrans = ht.HasTransparency
			}
			if hasTrans {
				pageData["has_transparency"] = true
			}

			annotCount := 0
			if ac, err := e.instance.FPDFPage_GetAnnotCount(&requests.FPDFPage_GetAnnotCount{Page: pageReq}); err == nil && ac != nil {
				annotCount = ac.Count
			}
			if annotCount > 0 {
				pageData["annotation_count"] = annotCount
			}

			defaultBoxes, _ := defaultLayout["boxes"].(map[string]any)
			if len(boxes) > 0 && !reflect.DeepEqual(boxes, defaultBoxes) {
				pageData["boxes"] = boxes
			}

			pages = append(pages, pageData)
		}
	} else {
		pages = make([]map[string]any, 0)
	}

	result := map[string]any{
		"path":                      pdfPath,
		"file_size_bytes":           st.Size(),
		"page_count":                pageCount,
		"is_encrypted":              secRev > 0,
		"is_tagged":                 isTagged,
		"has_valid_xref":            hasValidXref,
		"form_type":                 formType,
		"page_mode":                 pageMode,
		"security_handler_revision": secRev,
		"attachment_count":          attachmentCount,
		"javascript_action_count":   jsActionCount,
		"signature_count":           signatureCount,
		"metadata":                  metadata,
		"bookmarks":                 bookmarks,
		"pages":                     pages,
	}

	if defaultLayout != nil {
		result["page_layout"] = defaultLayout
	}
	if pdfVersion != "" {
		result["pdf_version"] = pdfVersion
	}
	if secRev > 0 || hasRestrictedPerm {
		result["permissions"] = permissions
	}
	if attachmentCount > 0 {
		result["attachments"] = attachments
		if attachmentCount > e.cfg.MaxAttachments {
			result["attachments_truncated"] = true
		}
	}
	if bookmarksTruncated {
		result["bookmarks_truncated"] = true
	}
	if len(pages) < pageCount {
		result["pages_returned"] = len(pages)
		result["pages_truncated"] = true
	}

	return result, nil
}

// RenderPage renders the specified page (or crop region) to PNG image bytes.
func (e *Engine) RenderPage(ctx context.Context, opts RenderOptions) (*RenderResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	left := 0.0
	top := 0.0
	right := 1.0
	bottom := 1.0

	if opts.Left != nil {
		left = *opts.Left
	}
	if opts.Top != nil {
		top = *opts.Top
	}
	if opts.Right != nil {
		right = *opts.Right
	}
	if opts.Bottom != nil {
		bottom = *opts.Bottom
	}

	if left < 0.0 || right > 1.0 || left >= right {
		return nil, fmt.Errorf("invalid horizontal crop bounds: left=%f, right=%f (must satisfy 0.0 <= left < right <= 1.0)", left, right)
	}
	if top < 0.0 || bottom > 1.0 || top >= bottom {
		return nil, fmt.Errorf("invalid vertical crop bounds: top=%f, bottom=%f (must satisfy 0.0 <= top < bottom <= 1.0)", top, bottom)
	}

	if err := e.ensureInit(); err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil, errors.New("engine is closed")
	}

	f, err := os.Open(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", opts.Path, err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %q: %w", opts.Path, err)
	}

	doc, err := e.instance.OpenDocument(&requests.OpenDocument{
		FileReader:     f,
		FileReaderSize: st.Size(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF document: %w", err)
	}
	defer e.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	pcResp, err := e.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}
	pageCount := pcResp.PageCount

	if opts.Page < 1 || opts.Page > pageCount {
		return nil, fmt.Errorf("page %d out of bounds (document has %d pages)", opts.Page, pageCount)
	}

	sizeResp, err := e.instance.FPDF_GetPageSizeByIndex(&requests.FPDF_GetPageSizeByIndex{
		Document: doc.Document,
		Index:    opts.Page - 1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get page size for page %d: %w", opts.Page, err)
	}

	cropFracW := right - left
	cropFracH := bottom - top

	var effectiveDPI int
	if opts.DPI != nil {
		if *opts.DPI <= 0 {
			return nil, errors.New("dpi must be > 0")
		}
		effectiveDPI = *opts.DPI
	} else {
		minFrac := math.Min(cropFracW, cropFracH)
		effectiveDPI = int(math.Round(float64(e.cfg.DefaultDPI) / minFrac))
	}
	if effectiveDPI < e.cfg.MinDPI {
		effectiveDPI = e.cfg.MinDPI
	} else if effectiveDPI > e.cfg.MaxDPI {
		effectiveDPI = e.cfg.MaxDPI
	}

	pxPerPt := float64(effectiveDPI) / PointsPerInch
	fullW := int(math.Ceil(sizeResp.Width * pxPerPt))
	fullH := int(math.Ceil(sizeResp.Height * pxPerPt))

	startX := int(math.Round(left * float64(fullW)))
	startY := int(math.Round(top * float64(fullH)))
	endX := int(math.Round(right * float64(fullW)))
	endY := int(math.Round(bottom * float64(fullH)))

	cropW := max(1, endX-startX)
	cropH := max(1, endY-startY)

	if cropW > e.cfg.MaxImageDimension || cropH > e.cfg.MaxImageDimension {
		downscale := float64(e.cfg.MaxImageDimension) / float64(max(cropW, cropH))
		pxPerPt *= downscale
		effectiveDPI = max(1, int(math.Round(float64(effectiveDPI)*downscale)))
		fullW = int(math.Round(sizeResp.Width * pxPerPt))
		fullH = int(math.Round(sizeResp.Height * pxPerPt))
		startX = int(math.Round(left * float64(fullW)))
		startY = int(math.Round(top * float64(fullH)))
		endX = int(math.Round(right * float64(fullW)))
		endY = int(math.Round(bottom * float64(fullH)))
		cropW = max(1, endX-startX)
		cropH = max(1, endY-startY)
		if cropW > e.cfg.MaxImageDimension {
			cropW = e.cfg.MaxImageDimension
		}
		if cropH > e.cfg.MaxImageDimension {
			cropH = e.cfg.MaxImageDimension
		}
	}

	bm, err := e.instance.FPDFBitmap_Create(&requests.FPDFBitmap_Create{
		Width:  cropW,
		Height: cropH,
		Alpha:  1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create bitmap: %w", err)
	}
	defer e.instance.FPDFBitmap_Destroy(&requests.FPDFBitmap_Destroy{Bitmap: bm.Bitmap})

	_, err = e.instance.FPDFBitmap_FillRect(&requests.FPDFBitmap_FillRect{
		Bitmap: bm.Bitmap,
		Left:   0,
		Top:    0,
		Width:  cropW,
		Height: cropH,
		Color:  OpaqueWhiteColor,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fill bitmap background: %w", err)
	}

	_, err = e.instance.FPDF_RenderPageBitmap(&requests.FPDF_RenderPageBitmap{
		Bitmap: bm.Bitmap,
		Page: requests.Page{
			ByIndex: &requests.PageByIndex{
				Document: doc.Document,
				Index:    opts.Page - 1,
			},
		},
		StartX: -startX,
		StartY: -startY,
		SizeX:  fullW,
		SizeY:  fullH,
		Rotate: enums.FPDF_PAGE_ROTATION_NONE,
		Flags:  enums.FPDF_RENDER_FLAG_ANNOT | enums.FPDF_RENDER_FLAG_LCD_TEXT | enums.FPDF_RENDER_FLAG_REVERSE_BYTE_ORDER,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render page bitmap: %w", err)
	}

	strideResp, err := e.instance.FPDFBitmap_GetStride(&requests.FPDFBitmap_GetStride{Bitmap: bm.Bitmap})
	if err != nil {
		return nil, fmt.Errorf("failed to get bitmap stride: %w", err)
	}

	bufResp, err := e.instance.FPDFBitmap_GetBuffer(&requests.FPDFBitmap_GetBuffer{Bitmap: bm.Bitmap})
	if err != nil {
		return nil, fmt.Errorf("failed to get bitmap buffer: %w", err)
	}

	img := &image.RGBA{
		Pix:    bufResp.Buffer,
		Stride: strideResp.Stride,
		Rect:   image.Rect(0, 0, cropW, cropH),
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}

	return &RenderResult{
		PNGBytes:     pngBuf.Bytes(),
		WidthPx:      cropW,
		HeightPx:     cropH,
		EffectiveDPI: effectiveDPI,
		Page:         opts.Page,
		TotalPages:   pageCount,
		Left:         left,
		Top:          top,
		Right:        right,
		Bottom:       bottom,
	}, nil
}

func toSnakeCase(s string) string {
	switch strings.ToLower(s) {
	case "title":
		return "title"
	case "author":
		return "author"
	case "subject":
		return "subject"
	case "keywords":
		return "keywords"
	case "creator":
		return "creator"
	case "producer":
		return "producer"
	case "creationdate", "creation_date":
		return "creation_date"
	case "moddate", "mod_date":
		return "mod_date"
	}

	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type paperSizeSpec struct {
	name     string
	widthPt  float64
	heightPt float64
}

var standardPaperSizes = []paperSizeSpec{
	{"A0", 2383.94, 3370.39},
	{"A1", 1683.78, 2383.94},
	{"A2", 1190.55, 1683.78},
	{"A3", 841.89, 1190.55},
	{"A4", 595.28, 841.89},
	{"A5", 419.53, 595.28},
	{"A6", 297.64, 419.53},
	{"B4", 708.66, 1000.63},
	{"B5", 498.90, 708.66},
	{"US Letter", 612.0, 792.0},
	{"US Legal", 612.0, 1008.0},
	{"Tabloid", 792.0, 1224.0},
	{"16:9 Presentation", 540.0, 960.0},
	{"16:9 Presentation", 405.0, 720.0},
	{"4:3 Presentation", 540.0, 720.0},
}

func detectPageFormat(w, h float64) string {
	minDim := math.Min(w, h)
	maxDim := math.Max(w, h)

	for _, spec := range standardPaperSizes {
		specMin := math.Min(spec.widthPt, spec.heightPt)
		specMax := math.Max(spec.widthPt, spec.heightPt)
		if math.Abs(minDim-specMin) <= PageFormatTolerancePt && math.Abs(maxDim-specMax) <= PageFormatTolerancePt {
			return spec.name
		}
	}

	if w > h && math.Abs((w/h)-(16.0/9.0)) <= AspectRatioTolerance {
		return "16:9 Presentation"
	}

	return "custom"
}
