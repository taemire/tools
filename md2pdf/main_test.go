package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"md2pdf/analyzer"
	"md2pdf/converter"
)

// Exercise the actual CLI (including os.Exit) without compiling another binary.
func TestCLIProcess(t *testing.T) {
	if os.Getenv("MD2PDF_CLI_TEST_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"md2pdf"}, os.Args[i+1:]...)
			flag.CommandLine = flag.NewFlagSet("md2pdf", flag.ExitOnError)
			main()
			os.Exit(0)
		}
	}
	t.Fatal("missing CLI arguments")
}

func runCLI(t *testing.T, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestCLIProcess$", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "MD2PDF_CLI_TEST_PROCESS=1")
	data, err := cmd.CombinedOutput()
	if err == nil {
		return string(data), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return string(data), exit.ExitCode()
	}
	t.Fatal(err)
	return "", -1
}

func TestPDFTOCIntegration(t *testing.T) {
	for _, template := range []string{"report", "default", "modern"} {
		t.Run(template, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "manual.md")
			longTitle := strings.Repeat("긴 제목 English API 설정 ", 8)
			titles := []string{"한국어 제목", "English heading", "AIRS 설정 및 API", longTitle, "References", "References"}
			// Titles also appear in the body before their headings and in the
			// generated TOC. None of these earlier text occurrences is a target.
			md := "# 같은 문서 Document\n\n<style>h2[id] { break-before: page !important; max-width: 300px; }</style>\n\n"
			md += "앞에서 언급한 제목: " + strings.Join(titles, " / ") + "\n\n"
			for _, title := range titles {
				md += "## " + title + "\n\n본문 Body text.\n\n"
			}
			if err := os.WriteFile(input, []byte(md), 0600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "result.pdf")
			log, code := runCLI(t, "-i", input, "-o", output, "-template", template, "-title", "같은 문서 Document")
			if code != 0 || strings.Contains(log, "[WARN]") || strings.Contains(log, "[ERROR]") {
				t.Fatalf("CLI exit=%d\n%s", code, log)
			}
			sectionsPath := filepath.Join(dir, "sections.json")
			_, err := converter.ConvertToHTML(converter.Options{InputDir: input, OutputFile: filepath.Join(dir, "inspect.html"), SectionsJSON: sectionsPath, Template: template, PDFMode: true})
			if err != nil {
				t.Fatal(err)
			}
			result, err := analyzer.AnalyzePDF(output, sectionsPath, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sections) != 7 {
				t.Fatalf("got %d entries, want 7", len(result.Sections))
			}
			// Each real heading is forced onto its own new page; the preview
			// paragraph and TOC must never capture the mapping.
			for i, section := range result.Sections {
				if section.Page != i+1 {
					t.Errorf("%q page=%d want=%d", section.Title, section.Page, i+1)
				}
			}
			data, err := os.ReadFile(strings.TrimSuffix(output, ".pdf") + ".html")
			if err != nil {
				t.Fatal(err)
			}
			pageSpans := regexp.MustCompile(`class="toc-page"[^>]*>\s*([0-9]+)\s*</span>`).FindAllStringSubmatch(string(data), -1)
			if len(pageSpans) != 7 {
				t.Fatalf("got %d numbered TOC entries, want 7", len(pageSpans))
			}
			for i, span := range pageSpans {
				if span[1] != strconv.Itoa(i+1) {
					t.Errorf("printed TOC[%d]=%s want=%d", i, span[1], i+1)
				}
			}
		})
	}
}

func TestPDFMissingDestinationPreservesExistingOutputs(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "manual.md"), filepath.Join(dir, "result.pdf")
	// A hidden heading has text and an HTML anchor but no PDF destination.
	if err := os.WriteFile(input, []byte("# Document\n\n<style>h2[id] { display: none }</style>\n\n## 숨긴 제목\n\n본문\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{output, filepath.Join(dir, "result.html")} {
		if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	log, code := runCLI(t, "-i", input, "-o", output)
	if code != 1 || !strings.Contains(log, "missing PDF destinations") {
		t.Fatalf("want mapping error and exit 1, got %d\n%s", code, log)
	}
	for _, path := range []string{output, filepath.Join(dir, "result.html")} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "existing output" {
			t.Fatalf("output changed on failure: %s %v", path, err)
		}
	}
	log, code = runCLI(t, "-i", input, "-o", filepath.Join(dir, "html-only.html"), "-html-only", "-offset", "10")
	if code != 0 {
		t.Fatalf("HTML-only compatibility failed: %d\n%s", code, log)
	}
}
