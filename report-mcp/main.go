package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"report-mcp/artexmd"
	"report-mcp/pentestreport"
)

const version = "0.3.0"

var sessionPattern = regexp.MustCompile(`^(conv|task)-[0-9]+$`)
var jobPattern = regexp.MustCompile(`^job-[a-f0-9]{32}$`)
var pagesPattern = regexp.MustCompile(`(?m)^Pages:\s+(\d+)`)

type config struct{ sessions, output, public, converter string }
type app struct {
	cfg              config
	sessions, output *os.Root
	busy             chan struct{}
	parseV3          func([]byte) (*artexmd.Document, error)
	nowV3            func() time.Time
	chromeExec       string // local CLI only: start this Chrome instead of REPORT_CHROME_URL
}
type renderInput struct {
	SessionID    string       `json:"session_id" jsonschema:"ARTEX session folder, for example conv-39"`
	MarkdownFile string       `json:"markdown_file,omitempty" jsonschema:"Markdown path relative to this session folder (legacy and artex-md-3)"`
	Title        string       `json:"title,omitempty" jsonschema:"Customer report title (legacy path)"`
	Author       string       `json:"author,omitempty" jsonschema:"Report author or company"`
	Graph        *reportGraph `json:"graph,omitempty" jsonschema:"Existing task relationships for a static customer diagram; no new discovery"`
	SourceFormat string       `json:"source_format,omitempty" jsonschema:"Omit for a session Markdown file (legacy md2pdf). artex-md-3: session Markdown with render_inputs. arx-pentest-report-1: source is the resolved arx-pentest-report JSON"`
	Formats      []string     `json:"formats,omitempty" jsonschema:"artex-md-3 only: html, pdf, docx"`
	Source       any          `json:"source,omitempty" jsonschema:"arx-pentest-report-1 only: the platform's resolved arx-pentest-report 1.x document (JSON object)"`
	// RawRenderInputs is decoded per source format (artexmd.RenderInputs or
	// pentestreport.RenderInputs).
	RawRenderInputs any                   `json:"render_inputs,omitempty" jsonschema:"Format-specific render inputs; see README"`
	RenderInputs    *artexmd.RenderInputs `json:"-"`
}
type resultInput struct {
	SessionID        string `json:"session_id"`
	JobID            string `json:"job_id"`
	CapabilitiesOnly bool   `json:"capabilities_only,omitempty"`
}
type receipt struct {
	SessionID       string            `json:"session_id"`
	JobID           string            `json:"job_id"`
	Status          string            `json:"status"`
	PDFPath         string            `json:"pdf_path"`
	HTMLPath        string            `json:"html_path"`
	DOCXPath        string            `json:"docx_path,omitempty"`
	PreviewPaths    []string          `json:"preview_paths"`
	PageCount       int               `json:"page_count"`
	SHA256          string            `json:"sha256"`
	SourceSHA256    string            `json:"source_sha256"`
	Checks          map[string]string `json:"checks"`
	CustomerRelease string            `json:"customer_release"`
	CreatedAt       string            `json:"created_at"`
	V3              *v3Receipt        `json:"v3,omitempty"`
	PentestReport   *pentestReceipt   `json:"arx_pentest_report,omitempty"`
	Capabilities    []string          `json:"capabilities,omitempty"`
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func newApp(c config) (*app, error) {
	s, err := os.OpenRoot(c.sessions)
	if err != nil {
		return nil, err
	}
	o, err := os.OpenRoot(c.output)
	if err != nil {
		s.Close()
		return nil, err
	}
	return &app{cfg: c, sessions: s, output: o, busy: make(chan struct{}, 1)}, nil
}
func (a *app) close() { a.sessions.Close(); a.output.Close() }

// validRel은 세션 안의 슬래시(/) 논리 경로만 허용한다. path.Clean으로 비교해야
// Windows에서도 filepath.Clean이 구분자를 바꿔 모든 하위 경로를 거부하지 않는다.
func validRel(s string) error {
	if !filepath.IsLocal(s) || s == "." || path.Clean(s) != s || strings.ContainsAny(s, "\x00\\") {
		return errors.New("path must be a clean relative path")
	}
	return nil
}
func boundedRead(root *os.Root, name string, max int64) ([]byte, error) {
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > max {
		return nil, errors.New("input must be a regular file within the size limit")
	}
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if int64(len(b)) > max {
		return nil, errors.New("input exceeded size limit")
	}
	return b, err
}

// Snapshot only session-local Markdown and PNG/JPEG/GIF image dependencies.
// Raw HTML and executable diagrams are refused; diagnostic tags belong in code fences.
func stageMarkdown(root *os.Root, rel string, dst string) ([]byte, error) {
	if err := validRel(rel); err != nil {
		return nil, err
	}
	if strings.ToLower(filepath.Ext(rel)) != ".md" {
		return nil, errors.New("only Markdown files are accepted")
	}
	b, err := boundedRead(root, rel, 2<<20)
	if err != nil {
		return nil, err
	}
	doc := goldmark.New().Parser().Parse(text.NewReader(b))
	images := map[string]bool{}
	var htmlRanges []text.Segment
	err = ast.Walk(doc, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}
		switch x := n.(type) {
		case *ast.RawHTML:
			for i := 0; i < x.Segments.Len(); i++ {
				htmlRanges = append(htmlRanges, x.Segments.At(i))
			}
		case *ast.HTMLBlock:
			for i := 0; i < x.Lines().Len(); i++ {
				htmlRanges = append(htmlRanges, x.Lines().At(i))
			}
			if x.HasClosure() {
				htmlRanges = append(htmlRanges, x.ClosureLine)
			}
		case *ast.FencedCodeBlock:
			if string(x.Language(b)) == "mermaid" {
				return ast.WalkStop, errors.New("convert Mermaid to a session-local PNG before rendering")
			}
		case *ast.Image:
			u, e := url.Parse(string(x.Destination))
			if e != nil || u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
				return ast.WalkStop, errors.New("images must be local session files")
			}
			p := path.Join(path.Dir(rel), u.Path)
			if e = validRel(p); e != nil {
				return ast.WalkStop, e
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" {
				return ast.WalkStop, errors.New("only local PNG/JPEG/GIF images are supported")
			}
			images[p] = true
		case *ast.Link:
			u, e := url.Parse(string(x.Destination))
			if e != nil {
				return ast.WalkStop, errors.New("invalid report link")
			}
			if u.Scheme != "" && u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "mailto" {
				return ast.WalkStop, errors.New("executable or local-file report links are refused")
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}
	if len(images) > 20 {
		return nil, errors.New("too many images")
	}
	for p := range images {
		data, e := boundedRead(root, p, 5<<20)
		if e != nil {
			return nil, fmt.Errorf("image %s: %w", p, e)
		}
		out := filepath.Join(dst, p)
		if e = os.MkdirAll(filepath.Dir(out), 0750); e != nil {
			return nil, e
		}
		if e = os.WriteFile(out, data, 0640); e != nil {
			return nil, e
		}
	}
	out := filepath.Join(dst, rel)
	if err = os.MkdirAll(filepath.Dir(out), 0750); err != nil {
		return nil, err
	}
	// Escape evidence markup instead of interpreting it as active report HTML.
	staged := append([]byte(nil), b...)
	sort.Slice(htmlRanges, func(i, j int) bool { return htmlRanges[i].Start > htmlRanges[j].Start })
	for _, seg := range htmlRanges {
		staged = append(append(append([]byte{}, staged[:seg.Start]...), []byte(html.EscapeString(string(b[seg.Start:seg.Stop])))...), staged[seg.Stop:]...)
	}
	if err = os.WriteFile(out, staged, 0640); err != nil {
		return nil, err
	}
	return b, nil
}
func run(ctx context.Context, name string, args ...string) (string, error) {
	return runEnv(ctx, nil, name, args...)
}

// runEnv runs a program with extra environment entries added to the service's.
func runEnv(ctx context.Context, extra []string, name string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, name, args...)
	if len(extra) > 0 {
		c.Env = append(os.Environ(), extra...)
	}
	c.WaitDelay = 3 * time.Second
	b, err := c.CombinedOutput()
	if err != nil {
		return string(b), fmt.Errorf("%s failed: %w", filepath.Base(name), err)
	}
	return string(b), nil
}

// embedHTMLFonts rewrites the HTML with data-URI fonts and returns the exact
// bytes written, which are the bytes the receipt's HTML hash covers.
func embedHTMLFonts(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := string(b)
	for _, name := range []string{"NanumGothic.ttf", "NanumGothicBold.ttf"} {
		file := "/usr/share/fonts/truetype/nanum/" + name
		needle := "file://" + file
		if !strings.Contains(s, needle) {
			continue
		}
		font, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		s = strings.ReplaceAll(s, needle, "data:font/ttf;base64,"+base64.StdEncoding.EncodeToString(font))
	}
	out := []byte(s)
	if err = os.WriteFile(path, out, 0640); err != nil {
		return nil, err
	}
	return out, nil
}
func bboxCheck(data []byte) (int, int, error) {
	d := xml.NewDecoder(strings.NewReader(string(data)))
	count, bad := 0, 0
	width, height := 0.0, 0.0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
		s, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		attrs := map[string]string{}
		for _, x := range s.Attr {
			attrs[x.Name.Local] = x.Value
		}
		if s.Name.Local == "page" {
			count++
			width, _ = strconv.ParseFloat(attrs["width"], 64)
			height, _ = strconv.ParseFloat(attrs["height"], 64)
		}
		if s.Name.Local == "word" {
			xmin, _ := strconv.ParseFloat(attrs["xMin"], 64)
			xmax, _ := strconv.ParseFloat(attrs["xMax"], 64)
			ymin, _ := strconv.ParseFloat(attrs["yMin"], 64)
			ymax, _ := strconv.ParseFloat(attrs["yMax"], 64)
			if xmin < -0.5 || ymin < -0.5 || xmax > width+.5 || ymax > height+.5 {
				bad++
			}
		}
	}
	if count == 0 {
		return 0, 0, errors.New("no PDF page bounds")
	}
	return count, bad, nil
}
func (a *app) save(r receipt) error {
	rel := filepath.Join(r.SessionID, r.JobID, "receipt.json")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err = a.output.WriteFile(rel+".tmp", append(b, '\n'), 0640); err != nil {
		return err
	}
	return a.output.Rename(rel+".tmp", rel)
}
func (a *app) load(in resultInput) (receipt, error) {
	var r receipt
	if !sessionPattern.MatchString(in.SessionID) || !jobPattern.MatchString(in.JobID) {
		return r, errors.New("invalid session or job id")
	}
	b, err := boundedRead(a.output, filepath.Join(in.SessionID, in.JobID, "receipt.json"), 1<<20)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(b, &r)
	return r, err
}
func (a *app) inspect(ctx context.Context, r *receipt) error {
	dir := filepath.Join(a.cfg.output, r.SessionID, r.JobID)
	pdf := filepath.Join(dir, "report.pdf")
	r.Checks = map[string]string{"content_review": "UNVERIFIED", "visual_review": "UNVERIFIED"}
	r.CustomerRelease = "HOLD_REQUIRES_CONTENT_AND_VISUAL_REVIEW"
	logText, err := run(ctx, "qpdf", "--check", pdf)
	os.WriteFile(filepath.Join(dir, "qpdf.log"), []byte(logText), 0640)
	if err != nil {
		r.Status = "HOLD"
		r.Checks["pdf_structure"] = "FAIL"
		return err
	}
	r.Checks["pdf_structure"] = "PASS"
	info, err := run(ctx, "pdfinfo", pdf)
	if err != nil {
		return err
	}
	m := pagesPattern.FindStringSubmatch(info)
	if len(m) != 2 {
		return errors.New("page count missing")
	}
	r.PageCount, _ = strconv.Atoi(m[1])
	if r.PageCount < 1 || r.PageCount > 80 {
		return errors.New("page count outside 1..80")
	}
	fonts, err := run(ctx, "pdffonts", pdf)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(fonts), "\n")
	if len(lines) < 3 {
		return errors.New("no embedded text font")
	}
	for _, line := range lines[2:] {
		f := strings.Fields(line)
		if len(f) < 6 || f[len(f)-5] != "yes" {
			return errors.New("PDF has an unembedded font")
		}
	}
	r.Checks["fonts_embedded"] = "PASS"
	if _, err = run(ctx, "pdftotext", "-layout", pdf, filepath.Join(dir, "report.txt")); err != nil {
		return err
	}
	if _, err = run(ctx, "pdftotext", "-bbox", pdf, filepath.Join(dir, "bounds.html")); err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(dir, "bounds.html"))
	if err != nil {
		return err
	}
	pages, bad, err := bboxCheck(b)
	if err != nil {
		return err
	}
	if pages != r.PageCount || bad != 0 {
		r.Status = "HOLD"
		r.Checks["text_bounds"] = fmt.Sprintf("FAIL_%d_OUTSIDE_PAGE", bad)
		return errors.New("PDF text extends beyond the page")
	}
	r.Checks["text_bounds"] = "PASS"
	if _, err = run(ctx, "pdftoppm", "-r", "85", "-png", pdf, filepath.Join(dir, "page")); err != nil {
		return err
	}
	previews, err := filepath.Glob(filepath.Join(dir, "page-*.png"))
	if err != nil {
		return err
	}
	sort.Strings(previews)
	if len(previews) != r.PageCount {
		return errors.New("incomplete page preview set")
	}
	r.PreviewPaths = nil
	for _, p := range previews {
		r.PreviewPaths = append(r.PreviewPaths, filepath.ToSlash(filepath.Join(a.cfg.public, r.SessionID, r.JobID, filepath.Base(p))))
	}
	data, err := os.ReadFile(pdf)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	r.SHA256 = hex.EncodeToString(sum[:])
	r.Status = "GENERATED_MACHINE_CHECKS_PASS"
	return nil
}
func (a *app) render(ctx context.Context, in renderInput) (receipt, error) {
	switch in.SourceFormat {
	case "":
		if in.Source != nil || in.RawRenderInputs != nil || in.RenderInputs != nil {
			return receipt{}, errors.New("SOURCE_FORMAT_REQUIRED: source or render_inputs was given without source_format; use \"artex-md-3\" or \"" + pentestreport.SourceFormat + "\", or send only markdown_file for the legacy Markdown path")
		}
	case "artex-md-3":
		if e := decodeV3Inputs(&in); e != nil {
			return receipt{}, e
		}
		return a.renderV3(ctx, in, "")
	case pentestreport.SourceFormat:
		return a.renderPentest(ctx, in)
	default:
		return receipt{}, unsupportedFormat(in.SourceFormat)
	}
	var r receipt
	if !sessionPattern.MatchString(in.SessionID) {
		return r, errors.New("session_id must match conv-N or task-N")
	}
	if len(in.Title) > 600 || len(in.Author) > 300 {
		return r, errors.New("metadata too long")
	}
	select {
	case a.busy <- struct{}{}:
		defer func() { <-a.busy }()
	default:
		return r, errors.New("renderer is busy; retry later")
	}
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	s, err := a.sessions.OpenRoot(in.SessionID)
	if err != nil {
		return r, err
	}
	defer s.Close()
	var id [16]byte
	if _, err = rand.Read(id[:]); err != nil {
		return r, err
	}
	r = receipt{SessionID: in.SessionID, JobID: "job-" + hex.EncodeToString(id[:]), Status: "RENDERING", CreatedAt: time.Now().UTC().Format(time.RFC3339), CustomerRelease: "HOLD_REQUIRES_CONTENT_AND_VISUAL_REVIEW"}
	rel := filepath.Join(r.SessionID, r.JobID)
	if err = a.output.MkdirAll(rel, 0750); err != nil {
		return r, err
	}
	dir := filepath.Join(a.cfg.output, rel)
	stage := filepath.Join(dir, "source")
	source, err := stageMarkdown(s, in.MarkdownFile, stage)
	if err != nil {
		return r, err
	}
	sum := sha256.Sum256(source)
	r.SourceSHA256 = hex.EncodeToString(sum[:])
	if in.Graph != nil && len(in.Graph.Nodes) > 0 {
		graphPath := filepath.Join(stage, filepath.Dir(in.MarkdownFile), "report-graph.png")
		if err = drawReportGraph(in.Graph, graphPath); err != nil {
			return r, err
		}
		mdPath := filepath.Join(stage, in.MarkdownFile)
		md, e := os.ReadFile(mdPath)
		if e != nil {
			return r, e
		}
		graphMD := "![주요 점검 흐름과 근거 관계](report-graph.png)"
		if strings.Contains(string(md), "[REPORT_GRAPH]") {
			md = []byte(strings.ReplaceAll(string(md), "[REPORT_GRAPH]", graphMD))
		} else {
			md = append(md, []byte("\n\n## 주요 점검 흐름\n\n"+graphMD+"\n")...)
		}
		if err = os.WriteFile(mdPath, md, 0640); err != nil {
			return r, err
		}
	}
	r.PDFPath = filepath.ToSlash(filepath.Join(a.cfg.public, rel, "report.pdf"))
	r.HTMLPath = filepath.ToSlash(filepath.Join(a.cfg.public, rel, "report.html"))
	args := []string{"-i", filepath.Join(stage, in.MarkdownFile), "-o", filepath.Join(dir, "report.pdf"), "-template", "customer", "-header", "고객 검토본", "-footer", "CONFIDENTIAL | 검토본 · 고객 제출 전 검증과 내용 확인 필요"}
	if in.Title != "" {
		args = append(args, "-title", in.Title)
	}
	if in.Author != "" {
		args = append(args, "-author", in.Author)
	}
	logs, err := runEnv(ctx, converterEnv(), a.cfg.converter, args...)
	// CommandContext may SIGKILL the converter on the MCP deadline, bypassing
	// its defer. Wait has returned; remove only this job's owned intermediates.
	if cleanupErr := cleanupConverterWorkspaces(dir); cleanupErr != nil {
		logs += "\n[ERROR] CONVERTER_WORKSPACE_CLEANUP_FAILED\n"
		if err == nil {
			err = errors.New("CONVERTER_WORKSPACE_CLEANUP_FAILED")
		}
	}
	os.WriteFile(filepath.Join(dir, "render.log"), []byte(logs), 0640)
	if err != nil || !strings.Contains(logs, "[VALIDATION] Checking final PDF destinations") {
		r.Status = "FAILED"
		r.Checks = map[string]string{"toc": "FAIL"}
		a.save(r)
		if err == nil {
			err = errors.New("final TOC validation was not observed")
		}
		return r, err
	}
	htmlBytes, err := embedHTMLFonts(filepath.Join(dir, "report.html"))
	if err != nil {
		return r, err
	}
	// Same field as artex-md-3: the chat download gate serves report.html only
	// when v3.html_sha256 matches the file. Only the hash is set; the other v3
	// fields stay empty (source_format ""), so the tool output still matches
	// the published schema and artex-md-3 consumers keep refusing this job.
	htmlSum := sha256.Sum256(htmlBytes)
	r.V3 = &v3Receipt{HTMLSHA256: hex.EncodeToString(htmlSum[:])}
	err = a.inspect(ctx, &r)
	r.Checks["toc"] = "PASS"
	if err != nil {
		r.Status = "HOLD"
		r.Checks["validation_error"] = err.Error()
	}
	if e := a.save(r); e != nil {
		return r, e
	}
	return r, err
}
func (a *app) server() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "report-pdf", Version: version}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "render_report_figures", Description: "Rasterize an ARTEX artex-layout-r3b-v1 SVG bundle using the pinned B1 renderer. No scanning or customer approval."}, func(ctx context.Context, _ *mcp.CallToolRequest, in reportFiguresInput) (*mcp.CallToolResult, reportFiguresResult, error) {
		r, e := a.renderReportFigures(ctx, in)
		return nil, r, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "render_report", Description: "Generate a customer PDF/HTML and all-page previews. source_format: omit for session Markdown (markdown_file); \"artex-md-3\" for slot Markdown with render_inputs; \"arx-pentest-report-1\" for the platform's resolved arx-pentest-report JSON in source with render_inputs. Other values (for example \"markdown\") are refused. Does not scan sites or certify report claims."}, func(ctx context.Context, _ *mcp.CallToolRequest, in renderInput) (*mcp.CallToolResult, receipt, error) {
		r, e := a.render(ctx, in)
		return nil, r, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "validate_report", Description: "Recheck a report generated by this service. Machine validation never implies customer approval."}, func(ctx context.Context, _ *mcp.CallToolRequest, in resultInput) (*mcp.CallToolResult, receipt, error) {
		select {
		case a.busy <- struct{}{}:
			defer func() { <-a.busy }()
		default:
			return nil, receipt{}, errors.New("renderer is busy")
		}
		r, e := a.load(in)
		if e != nil {
			return nil, r, e
		}
		ctx, c := context.WithTimeout(ctx, 120*time.Second)
		defer c()
		e = a.inspect(ctx, &r)
		if e != nil {
			r.Status = "HOLD"
			r.Checks["validation_error"] = e.Error()
		}
		if err := a.save(r); err != nil {
			return nil, r, err
		}
		return nil, r, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "get_report_result", Description: "Get generated file paths, hashes and machine checks. Content/visual customer review remains separate."}, func(_ context.Context, _ *mcp.CallToolRequest, in resultInput) (*mcp.CallToolResult, receipt, error) {
		if in.CapabilitiesOnly {
			return nil, receipt{Status: "CAPABILITIES", Checks: map[string]string{}, PreviewPaths: []string{}, Capabilities: []string{"artex-md-3", "docx", "render_report_figures", pentestreport.SourceFormat, pentestreport.SourceFormat + ":docx"}}, nil
		}
		r, e := a.load(in)
		return nil, r, e
	})
	return s
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "render-v3" {
		if e := renderV3CLI(os.Args[2:]); e != nil {
			log.Print(e)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "wm-position-check" {
		if e := positionCLI(os.Args[2:]); e != nil {
			log.Print(e)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "render-pentest" {
		if e := pentestCLI(os.Args[2:]); e != nil {
			log.Print(e)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "render-figures" {
		if err := figuresCLI(os.Args[2:]); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	listen := env("REPORT_LISTEN", "127.0.0.1:8891")
	host, _, err := net.SplitHostPort(listen)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("REPORT_LISTEN must use a numeric loopback address")
	}
	a, err := newApp(config{env("REPORT_SESSION_ROOT", "/opt/artex-lab/data/sessions"), env("REPORT_OUTPUT_ROOT", "/opt/artex-lab/data/report-pdf"), env("REPORT_PUBLIC_ROOT", "/app/data/report-pdf"), env("MD2PDF_BIN", "/opt/artex-lab/report-mcp/bin/md2pdf")})
	if err != nil {
		log.Fatal(err)
	}
	defer a.close()
	for _, bin := range []string{a.cfg.converter, "google-chrome", "qpdf", "pdfinfo", "pdffonts", "pdftotext", "pdftoppm"} {
		if _, e := exec.LookPath(bin); e != nil {
			log.Fatalf("required program unavailable: %s", bin)
		}
	}
	s := a.server()
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "service": "report-pdf", "version": version, "customer_release": "requires review"})
	})
	log.Printf("report-pdf %s listening on %s", version, listen)
	log.Fatal((&http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 240 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}).ListenAndServe())
}

// converterEnv tells md2pdf how to reach Chrome: reuse the resident Chrome at
// REPORT_CHROME_URL when it answers, otherwise start one on the firewall-allowed
// port 9223 bound to 127.0.0.1 after checking the port is free. md2pdf pins the
// WebSocket address to that IPv4 endpoint, so a Chrome that falls back to
// [::1] is never dialled (production failure of 2026-11-02).
func converterEnv() []string {
	return []string{"MD2PDF_CHROME_URL=" + env("REPORT_CHROME_URL", "http://127.0.0.1:9223"), "MD2PDF_DEVTOOLS_PORT=" + env("REPORT_MD2PDF_DEVTOOLS_PORT", "9223")}
}

// cleanupConverterWorkspaces is called only after the child process exits.
// ReadDir avoids interpreting output-root characters as a glob pattern.
func cleanupConverterWorkspaces(jobDir string) error {
	entries, err := os.ReadDir(jobDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".md2pdf-") {
			if err := os.RemoveAll(filepath.Join(jobDir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
