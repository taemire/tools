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
)

const version = "0.1.0"

var sessionPattern = regexp.MustCompile(`^(conv|task)-[0-9]+$`)
var jobPattern = regexp.MustCompile(`^job-[a-f0-9]{32}$`)
var pagesPattern = regexp.MustCompile(`(?m)^Pages:\s+(\d+)`)

type config struct{ sessions, output, public, converter string }
type app struct {
	cfg              config
	sessions, output *os.Root
	busy             chan struct{}
}
type renderInput struct {
	SessionID    string       `json:"session_id" jsonschema:"ARTEX session folder, for example conv-39"`
	MarkdownFile string       `json:"markdown_file" jsonschema:"Markdown path relative to this session folder"`
	Title        string       `json:"title" jsonschema:"Customer report title"`
	Author       string       `json:"author,omitempty" jsonschema:"Report author or company"`
	Graph        *reportGraph `json:"graph,omitempty" jsonschema:"Existing task relationships for a static customer diagram; no new discovery"`
}
type resultInput struct {
	SessionID string `json:"session_id"`
	JobID     string `json:"job_id"`
}
type receipt struct {
	SessionID       string            `json:"session_id"`
	JobID           string            `json:"job_id"`
	Status          string            `json:"status"`
	PDFPath         string            `json:"pdf_path"`
	HTMLPath        string            `json:"html_path"`
	PreviewPaths    []string          `json:"preview_paths"`
	PageCount       int               `json:"page_count"`
	SHA256          string            `json:"sha256"`
	SourceSHA256    string            `json:"source_sha256"`
	Checks          map[string]string `json:"checks"`
	CustomerRelease string            `json:"customer_release"`
	CreatedAt       string            `json:"created_at"`
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
func validRel(s string) error {
	if !filepath.IsLocal(s) || s == "." || filepath.Clean(s) != s || strings.ContainsAny(s, "\x00\\") {
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
			p := filepath.Clean(filepath.Join(filepath.Dir(rel), u.Path))
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
	c := exec.CommandContext(ctx, name, args...)
	c.WaitDelay = 3 * time.Second
	b, err := c.CombinedOutput()
	if err != nil {
		return string(b), fmt.Errorf("%s failed: %w", filepath.Base(name), err)
	}
	return string(b), nil
}

func embedHTMLFonts(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
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
			return err
		}
		s = strings.ReplaceAll(s, needle, "data:font/ttf;base64,"+base64.StdEncoding.EncodeToString(font))
	}
	return os.WriteFile(path, []byte(s), 0640)
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
	logs, err := run(ctx, a.cfg.converter, args...)
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
	if err = embedHTMLFonts(filepath.Join(dir, "report.html")); err != nil {
		return r, err
	}
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
	mcp.AddTool(s, &mcp.Tool{Name: "render_report", Description: "Generate a customer PDF/HTML and all-page previews from existing session Markdown. Does not scan sites or certify report claims."}, func(ctx context.Context, _ *mcp.CallToolRequest, in renderInput) (*mcp.CallToolResult, receipt, error) {
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
		r, e := a.load(in)
		return nil, r, e
	})
	return s
}
func main() {
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
