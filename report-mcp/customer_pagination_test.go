package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"golang.org/x/net/html"
)

type paginationWord struct {
	XMin float64 `xml:"xMin,attr"`
	YMin float64 `xml:"yMin,attr"`
	XMax float64 `xml:"xMax,attr"`
	YMax float64 `xml:"yMax,attr"`
	Text string  `xml:",chardata"`
}
type paginationPage struct {
	Width, Height float64
	Words         []paginationWord
}
type paginationPosition struct {
	Page int
	Word paginationWord
}

func paginationHash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func paginationCompact(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}
func paginationPDFPages(t *testing.T, data []byte) []paginationPage {
	t.Helper()
	d := xml.NewDecoder(bytes.NewReader(data))
	var pages []paginationPage
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		s, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if s.Name.Local == "page" {
			p := paginationPage{}
			for _, a := range s.Attr {
				switch a.Name.Local {
				case "width":
					p.Width, _ = strconv.ParseFloat(a.Value, 64)
				case "height":
					p.Height, _ = strconv.ParseFloat(a.Value, 64)
				}
			}
			pages = append(pages, p)
		}
		if s.Name.Local == "word" {
			if len(pages) == 0 {
				t.Fatal("word without PDF page")
			}
			var w paginationWord
			if err := d.DecodeElement(&w, &s); err != nil {
				t.Fatal(err)
			}
			pages[len(pages)-1].Words = append(pages[len(pages)-1].Words, w)
		}
	}
	if len(pages) == 0 {
		t.Fatal("no PDF pages")
	}
	return pages
}

// The current worktree template is rebuilt, then one real converter/app render
// checks content and PDF geometry. No cached converter or CSS-string assertion
// can turn this test green. Chrome and Poppler are required, never skipped.
func TestCustomerParagraphPaginationNative(t *testing.T) {
	for _, name := range []string{"go", "patch", "qpdf", "pdfinfo", "pdffonts", "pdftotext", "pdftoppm"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("required native tool %s: %v", name, err)
		}
	}
	chrome := env("REPORT_TEST_CHROME", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
	if _, err := os.Stat(chrome); err != nil {
		t.Fatal("local Chrome required", err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	regular := mustAbs(t, "pentestreport/assets/fonts/Pretendard-Regular.ttf")
	bold := mustAbs(t, "pentestreport/assets/fonts/Pretendard-Bold.ttf")
	for _, p := range []string{regular, bold} {
		if _, err := os.Stat(p); err != nil {
			t.Fatal("trusted bundled font required", err)
		}
	}
	converter := localCustomerConverter(t, root, regular, bold)
	privateTmp := filepath.Join(root, "unused-private-tmp")
	if err := os.Mkdir(privateTmp, 0700); err != nil {
		t.Fatal(err)
	}
	endpoint, _ := residentWithPrivateTmpDenied(t, root, privateTmp)
	t.Setenv("REPORT_CHROME_URL", endpoint)
	t.Setenv("REPORT_MD2PDF_DEVTOOLS_PORT", strings.TrimPrefix(endpoint, "http://127.0.0.1:"))
	markers := []string{}
	bodies := []string{}
	paragraph := func(prefix string, n int) string {
		first, last := prefix+"BEGIN", prefix+"END"
		markers = append(markers, first, last)
		var b strings.Builder
		b.WriteString(first + " ")
		for i := 0; b.Len()+len(last)+25 < n; i++ {
			token := fmt.Sprintf("%s%04d", prefix, i)
			markers = append(markers, token)
			fmt.Fprintf(&b, "%s alpha beta ", token)
		}
		b.WriteString(strings.Repeat("x", n-b.Len()-len(last)-1))
		b.WriteString(" " + last)
		if len([]rune(b.String())) != n {
			t.Fatal("fixture paragraph length mismatch")
		}
		bodies = append(bodies, b.String())
		return b.String()
	}
	longP, largeLI := paragraph("P", 7099), paragraph("L", 7099)
	var md strings.Builder
	md.WriteString("# H1KEEP\n\nH1FIRST 정상 진단 short introduction.\n\n## H2PARAGRAPH\n\nH2FIRST short paragraph before the long paragraph.\n\n" + longP + "\n\n### H3FIRSTKEEP\n\nH3FIRSTP first short paragraph.\n\nH3SECONDP second short paragraph.\n\n#### H4KEEP\n\nH4FIRST short paragraph following H4.\n\n## H2LISTS\n\nLISTINTRO bounded list content.\n\n")
	markers = append(markers, "H1FIRST", "H2FIRST", "H3FIRSTP", "H3SECONDP", "H4FIRST", "LISTINTRO")
	for i := 0; i < 65; i++ {
		token := fmt.Sprintf("U%03d", i)
		markers = append(markers, token)
		fmt.Fprintf(&md, "- %s short list item preserved in original order.\n", token)
	}
	md.WriteString("\n- " + largeLI + "\n\n- NESTPARENT nested evidence observations.\n")
	markers = append(markers, "NESTPARENT")
	for i := 0; i < 12; i++ {
		token := fmt.Sprintf("N%03d", i)
		markers = append(markers, token)
		fmt.Fprintf(&md, "  - %s nested list item.\n", token)
	}
	md.WriteString("\n### H3SECONDKEEP\n\nH3NEXTFIRST first short paragraph after lists.\n\nH3NEXTSECOND second short paragraph after lists.\n\n## H2EVIDENCE\n\nPREINTRO preserved preformatted evidence below.\n\n```text\n")
	markers = append(markers, "H3NEXTFIRST", "H3NEXTSECOND", "PREINTRO")
	for i := 0; i < 90; i++ {
		token := fmt.Sprintf("E%03d", i)
		markers = append(markers, token)
		fmt.Fprintf(&md, "%s HTTP/1.1 200 synthetic evidence line %03d\n", token, i)
	}
	md.WriteString("```\n\n### H3FINALKEEP\n\nFINALFIRST first paragraph following evidence.\n\nFINALSECOND second paragraph following evidence.\n\nDOCUMENTEND\n")
	markers = append(markers, "FINALFIRST", "FINALSECOND", "DOCUMENTEND")
	source := []byte(md.String())
	sessions := filepath.Join(root, "sessions")
	session := filepath.Join(sessions, "task-90001")
	out := filepath.Join(root, "report-pdf")
	for _, p := range []string{session, out} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	input := filepath.Join(session, "pagination.md")
	if err := os.WriteFile(input, source, 0600); err != nil {
		t.Fatal(err)
	}
	a, err := newApp(config{sessions, out, out, converter})
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r, renderErr := a.render(ctx, renderInput{SessionID: "task-90001", MarkdownFile: "pagination.md", Title: "Native pagination regression"})
	job := filepath.Join(out, r.SessionID, r.JobID)
	// Export both success and failure artifacts, without altering the source or PDF.
	if evidence := os.Getenv("REPORT_PAGINATION_EVIDENCE"); evidence != "" {
		defer func() {
			if err := os.MkdirAll(evidence, 0700); err != nil {
				t.Error(err)
				return
			}
			_ = os.WriteFile(filepath.Join(evidence, "fixture.md"), source, 0600)
			entries, _ := os.ReadDir(job)
			for _, entry := range entries {
				if !entry.IsDir() {
					b, err := os.ReadFile(filepath.Join(job, entry.Name()))
					if err == nil {
						_ = os.WriteFile(filepath.Join(evidence, entry.Name()), b, 0600)
					}
				}
			}
			template, _ := os.ReadFile("templates/layout_customer.html")
			binary, _ := os.ReadFile(converter)
			reg, _ := os.ReadFile(regular)
			heavy, _ := os.ReadFile(bold)
			b, _ := json.MarshalIndent(map[string]any{"source_sha256": paginationHash(source), "template_sha256": paginationHash(template), "converter_sha256": paginationHash(binary), "regular_font_sha256": paginationHash(reg), "bold_font_sha256": paginationHash(heavy), "pages": r.PageCount, "markers": len(markers), "paragraph_characters": 7099, "list_item_characters": 7099, "failed": t.Failed()}, "", "  ")
			_ = os.WriteFile(filepath.Join(evidence, "regression.json"), b, 0600)
		}()
	}
	if renderErr != nil {
		log, _ := os.ReadFile(filepath.Join(job, "render.log"))
		t.Fatalf("native app.render: %v\n%s", renderErr, log)
	}
	if r.Status != "GENERATED_MACHINE_CHECKS_PASS" || r.Checks["fonts_embedded"] != "PASS" || r.Checks["toc"] != "PASS" || r.Checks["text_bounds"] != "PASS" {
		t.Fatalf("native inspection did not pass: %+v", r)
	}
	after, err := os.ReadFile(input)
	if err != nil || !bytes.Equal(after, source) || r.SourceSHA256 != paginationHash(source) {
		t.Fatal("source Markdown bytes/hash changed", err)
	}
	bbox, err := exec.Command("pdftotext", "-bbox-layout", r.PDFPath, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(job, "pagination-bbox.xml"), bbox, 0600)
	pages := paginationPDFPages(t, bbox)
	if len(pages) != r.PageCount {
		t.Fatal("receipt/page count mismatch")
	}
	positions := map[string][]paginationPosition{}
	var textAll strings.Builder
	for pi, p := range pages {
		meaningful := 0
		for _, w := range p.Words {
			body := w.YMin < p.Height-45
			if body {
				meaningful++
				textAll.WriteString(w.Text)
				textAll.WriteByte(' ')
			}
			if w.XMin < -.5 || w.YMin < -.5 || w.XMax > p.Width+.5 || w.YMax > p.Height+.5 {
				t.Errorf("page %d word outside page: %+v", pi+1, w)
			}
			if body && (w.XMin < 44 || w.XMax > p.Width-44 || w.YMin < 49 || w.YMax > p.Height-54) {
				t.Errorf("page %d body outside print margins: %+v", pi+1, w)
			}
			positions[w.Text] = append(positions[w.Text], paginationPosition{pi + 1, w})
		}
		if meaningful == 0 {
			t.Errorf("page %d contains only footer/page number (blank content page)", pi+1)
		}
		for i, w := range p.Words {
			for _, v := range p.Words[i+1:] {
				ox := math.Min(w.XMax, v.XMax) - math.Max(w.XMin, v.XMin)
				oy := math.Min(w.YMax, v.YMax) - math.Max(w.YMin, v.YMin)
				if ox > 1.5 && oy > math.Min(w.YMax-w.YMin, v.YMax-v.YMin)*.5 {
					t.Errorf("page %d overlapping text: %q / %q", pi+1, w.Text, v.Text)
				}
			}
		}
	}
	for _, marker := range markers {
		if n := len(positions[marker]); n != 1 {
			t.Errorf("source marker %s occurs %d times (wanted once)", marker, n)
		}
	}
	compact := paginationCompact(textAll.String())
	for _, body := range bodies {
		if !strings.Contains(compact, paginationCompact(body)) {
			t.Error("long paragraph/list item lost or reordered source characters")
		}
	}
	for _, prefix := range []string{"P", "L"} {
		begin, end := positions[prefix+"BEGIN"], positions[prefix+"END"]
		if len(begin) == 1 && len(end) == 1 && end[0].Page <= begin[0].Page {
			t.Errorf("%s long flow did not span pages", prefix)
		}
	}
	for heading, next := range map[string][]string{"H1KEEP": {"H1FIRST"}, "H2PARAGRAPH": {"H2FIRST"}, "H3FIRSTKEEP": {"H3FIRSTP", "H3SECONDP"}, "H4KEEP": {"H4FIRST"}, "H2LISTS": {"LISTINTRO"}, "H3SECONDKEEP": {"H3NEXTFIRST", "H3NEXTSECOND"}, "H2EVIDENCE": {"PREINTRO"}, "H3FINALKEEP": {"FINALFIRST", "FINALSECOND"}} {
		hp := positions[heading]
		if len(hp) == 0 {
			t.Errorf("heading %s missing", heading)
			continue
		}
		h := hp[len(hp)-1]
		for _, name := range next {
			np := positions[name]
			if len(np) != 1 || np[0].Page != h.Page || np[0].Word.YMin <= h.Word.YMin {
				t.Errorf("heading %s orphaned from %s", heading, name)
			}
		}
	}
	htmlBytes, err := os.ReadFile(r.HTMLPath)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatal(err)
	}
	attr := func(n *html.Node, key string) string {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val
			}
		}
		return ""
	}
	var nodeText func(*html.Node) string
	nodeText = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}
		s := ""
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			s += nodeText(c)
		}
		return s
	}
	toc := map[string]int{}
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, inTOC bool) {
		inTOC = inTOC || (n.Type == html.ElementNode && n.Data == "nav" && attr(n, "class") == "toc")
		if inTOC && n.Type == html.ElementNode && n.Data == "li" {
			title, number := "", ""
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Data == "a" {
					title = strings.TrimSpace(nodeText(c))
				}
				if c.Data == "span" && attr(c, "class") == "toc-page" {
					number = strings.TrimSpace(nodeText(c))
				}
			}
			p, err := strconv.Atoi(number)
			if err != nil || p <= 0 {
				t.Errorf("TOC has no positive page: %s %q", title, number)
			}
			toc[title] = p
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inTOC)
		}
	}
	walk(doc, false)
	for _, heading := range []string{"H1KEEP", "H2PARAGRAPH", "H2LISTS", "H2EVIDENCE"} {
		occ := positions[heading]
		if len(occ) != 2 || toc[heading] != occ[len(occ)-1].Page {
			t.Errorf("TOC page/body destination mismatch for %s: toc=%d positions=%v", heading, toc[heading], occ)
		}
	}
	fonts, err := exec.Command("pdffonts", r.PDFPath).Output()
	if err != nil || !bytes.Contains(fonts, []byte("Pretendard")) || bytes.Contains(fonts, []byte("Type 3")) {
		t.Errorf("trusted text fonts not embedded correctly: %v %s", err, fonts)
	}
	for _, prefix := range []string{"P", "L", "U", "N", "E"} {
		var seq []string
		for _, marker := range markers {
			if strings.HasPrefix(marker, prefix) && len(marker) > 1 && marker[1] >= '0' && marker[1] <= '9' {
				seq = append(seq, marker)
			}
		}
		sort.Strings(seq)
		last := -1
		all := textAll.String()
		for _, marker := range seq {
			index := strings.Index(all, marker)
			if index <= last {
				t.Errorf("numbered %s flow reordered at %s", prefix, marker)
			}
			last = index
		}
	}
	t.Logf("native current-template PDF: pages=%d markers=%d source=%s", r.PageCount, len(markers), r.SourceSHA256)
}
