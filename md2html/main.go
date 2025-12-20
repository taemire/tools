package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	BuildVersion = "1.0.0"
	BuildTime    = ""
)

// ManualSection represents a section of the manual
type ManualSection struct {
	Title   string
	ID      string
	Content string
	Level   int
}

// ManualConfig defines which files to include
type ManualConfig struct {
	Title    string
	Subtitle string
	Version  string
	Date     string
	Author   string
	Sections []ManualSection
}

func main() {
	inputDir := flag.String("i", "", "Input directory containing markdown files")
	outputFile := flag.String("o", "", "Output HTML file path")
	title := flag.String("title", "TSGroup Code Signing Service", "Main title (service name)")
	subtitle := flag.String("subtitle", "", "Subtitle (document type, e.g. 사용자 매뉴얼)")
	version := flag.String("version", "0.3.0", "Document version")
	author := flag.String("author", "", "Author/Company name (optional)")
	var templateName string
	flag.StringVar(&templateName, "template", "default", "Template name (see available templates below)")
	flag.StringVar(&templateName, "t", "default", "Template name (shorthand)")

	showVersion := flag.Bool("v", false, "Show version")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: md2html -i <input_dir> -o <output.html> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nAvailable Templates:\n")
		printTemplates()
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("md2html v%s (%s)\n", BuildVersion, BuildTime)
		return
	}

	if *inputDir == "" || *outputFile == "" {
		fmt.Println("Usage: md2html -i <input_dir> -o <output.html> [options]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -i string       Input directory with markdown files")
		fmt.Println("  -o string       Output HTML file")
		fmt.Println("  -title string   Document title (default: 사용자 매뉴얼)")
		fmt.Println("  -version string Document version (default: 0.3.0)")
		fmt.Println("  -template string Template name (default: default)")
		fmt.Println("  -v              Show version")
		os.Exit(1)
	}

	// Find sidebar to determine file order
	sidebarPath := filepath.Join(*inputDir, "_sidebar.md")
	files, err := parseSidebar(sidebarPath, *inputDir)
	if err != nil {
		fmt.Printf("[WARN] Could not parse sidebar, scanning directory: %v\n", err)
		files, _ = scanMarkdownFiles(*inputDir)
	}

	fmt.Printf("[INFO] Found %d markdown files\n", len(files))

	// Parse and convert each markdown file
	var sections []ManualSection
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Table),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("[WARN] Could not read %s: %v\n", file, err)
			continue
		}

		var buf bytes.Buffer
		if err := md.Convert(content, &buf); err != nil {
			fmt.Printf("[WARN] Could not convert %s: %v\n", file, err)
			continue
		}

		// Post-process: Convert mermaid code blocks to mermaid divs
		htmlContent := convertMermaidBlocks(buf.String())
		htmlContent = rewriteAssetPaths(htmlContent)

		// Extract title from first heading
		titleText := extractTitle(string(content))
		id := generateID(file)

		sections = append(sections, ManualSection{
			Title:   titleText,
			ID:      id,
			Content: htmlContent,
			Level:   1,
		})
	}

	// Generate HTML
	// Generate HTML
	htmlContent, err := generateHTML(*title, *subtitle, *version, *author, templateName, sections)
	if err != nil {
		fmt.Printf("[ERROR] Failed to generate HTML: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if err := os.WriteFile(*outputFile, []byte(htmlContent), 0644); err != nil {
		fmt.Printf("[ERROR] Failed to write output: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[SUCCESS] Generated: %s\n", *outputFile)
}

func parseSidebar(sidebarPath, baseDir string) ([]string, error) {
	content, err := os.ReadFile(sidebarPath)
	if err != nil {
		return nil, err
	}

	// Parse markdown links: [text](/path/to/file.md)
	re := regexp.MustCompile(`\[([^\]]+)\]\((/[^)]+\.md)\)`)
	matches := re.FindAllStringSubmatch(string(content), -1)

	var files []string
	for _, match := range matches {
		if len(match) >= 3 {
			path := strings.TrimPrefix(match[2], "/")
			fullPath := filepath.Join(baseDir, path)
			if _, err := os.Stat(fullPath); err == nil {
				files = append(files, fullPath)
			}
		}
	}

	return files, nil
}

func scanMarkdownFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") && !strings.HasPrefix(info.Name(), "_") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func extractTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return "Untitled"
}

func generateID(filePath string) string {
	base := filepath.Base(filePath)
	base = strings.TrimSuffix(base, ".md")
	base = strings.ReplaceAll(base, " ", "-")
	return strings.ToLower(base)
}

// convertMermaidBlocks converts <pre><code class="language-mermaid"> to <div class="mermaid">
func convertMermaidBlocks(html string) string {
	// Pattern: <pre><code class="language-mermaid">...</code></pre>
	re := regexp.MustCompile(`(?s)<pre><code class="language-mermaid">(.*?)</code></pre>`)
	return re.ReplaceAllString(html, `<div class="mermaid">$1</div>`)
}

// rewriteAssetPaths normalizes asset paths to be relative to the output HTML
func rewriteAssetPaths(html string) string {
	// Pattern: src="../../assets/..." -> src="assets/..."
	re := regexp.MustCompile(`src="(?:\.\./)+assets/`)
	return re.ReplaceAllString(html, `src="assets/`)
}

//go:embed templates/*.html
var templateFS embed.FS

func generateHTML(title, subtitle, version, author, templateName string, sections []ManualSection) (string, error) {
	filename := "templates/layout.html"
	if templateName != "default" && templateName != "" {
		filename = fmt.Sprintf("templates/layout_%s.html", templateName)
	}

	tmplData, err := templateFS.ReadFile(filename)
	if err != nil {
		return "", err
	}

	funcMap := template.FuncMap{
		"inc": func(i int) int { return i + 1 },
		"slice": func(s string, start, end int) string {
			if len(s) < end {
				return s
			}
			return s[start:end]
		},
	}

	t, err := template.New("manual").Funcs(funcMap).Parse(string(tmplData))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	data := ManualConfig{
		Title:    title,
		Subtitle: subtitle,
		Version:  version,
		Date:     findDate(),
		Author:   author,
		Sections: sections,
	}

	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func printTemplates() {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "layout") || !strings.HasSuffix(name, ".html") {
			continue
		}

		tplName := "default"
		if name != "layout.html" {
			// layout_Foo.html -> Foo
			tplName = strings.TrimPrefix(name, "layout_")
			tplName = strings.TrimSuffix(tplName, ".html")
		}
		fmt.Fprintf(os.Stderr, "  %s\n", tplName)
	}
}

func findDate() string {
	return time.Now().Format("2006년 01월 02일")
}
