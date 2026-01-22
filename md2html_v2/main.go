package main

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	gohtml "html"
	"mime"
	"net/http"
	"net/url"
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
	"gopkg.in/yaml.v3"
)

// AuthorsConfig는 AUTHORS.yml 파일 구조
type AuthorsConfig struct {
	ProjectName  string `yaml:"project_name"`
	Organization string `yaml:"organization"`
	Copyright    string `yaml:"copyright"`
	Document     struct {
		Title    string `yaml:"title"`
		Subtitle string `yaml:"subtitle"`
		Author   string `yaml:"author"`
		Header   string `yaml:"header"`
		Footer   string `yaml:"footer"`
	} `yaml:"document"`
}

var (
	BuildVersion = "1.0.0"
	BuildTime    = ""
)

// SubHeading represents a sub-heading within a section (H2, H3, etc.)
type SubHeading struct {
	Title      string `json:"title"`
	ID         string `json:"id"`
	Level      int    `json:"level"`          // 2 for H2, 3 for H3, etc.
	PageNumber int    `json:"page,omitempty"` // 2-Pass에서 사용
}

// ManualSection represents a section of the manual
type ManualSection struct {
	Title       string       `json:"title"`
	ID          string       `json:"id"`
	Content     string       `json:"-"` // JSON에서 제외 (너무 큼)
	Level       int          `json:"level"`
	SubHeadings []SubHeading `json:"subheadings,omitempty"`
	PageNumber  int          `json:"page,omitempty"` // 2-Pass에서 사용
}

// ManualConfig defines which files to include
type ManualConfig struct {
	Title     string
	Subtitle  string
	Version   string
	Date      string
	Author    string
	Header    string // PDF 헤더 텍스트
	Footer    string // PDF 푸터 텍스트
	Copyright string
	Sections  []ManualSection
}

func main() {
	// Parse flags.
	inputDir := flag.String("i", "", "Input directory containing markdown files")
	outputFile := flag.String("o", "", "Output HTML file path")

	// GNU 스타일: -c / --config
	var configFile string
	flag.StringVar(&configFile, "c", "", "Config file path (AUTHORS.yml)")
	flag.StringVar(&configFile, "config", "", "Config file path (AUTHORS.yml)")

	title := flag.String("title", "", "Main title (overrides config)")
	subtitle := flag.String("subtitle", "", "Subtitle (overrides config)")
	version := flag.String("version", "", "Document version")
	author := flag.String("author", "", "Author/Company name (overrides config)")
	header := flag.String("header", "", "Header text for printed pages (overrides config)")
	footer := flag.String("footer", "", "Footer text for printed pages (overrides config)")

	var templateName string
	flag.StringVar(&templateName, "template", "default", "Template name")
	flag.StringVar(&templateName, "t", "default", "Template name (shorthand)")
	embedImgs := flag.Bool("embed", true, "Embed images as Base64")
	pdfMode := flag.Bool("pdf-mode", false, "Rewrite .md links to internal anchors for PDF")

	// 2-Pass PDF 생성 옵션
	sectionsJSONOut := flag.String("sections-json", "", "Output sections list as JSON (for 2-pass PDF)")
	pagesJSONIn := flag.String("pages-json", "", "Input page numbers JSON (from PDF analyzer)")

	showVersion := flag.Bool("v", false, "Show version")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: md2html -i <input_dir> -o <output.html> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nAvailable Templates:\n")
		printTemplates()
	}

	flag.Parse()

	// 설정 파일에서 값 로드
	var cfg AuthorsConfig
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			fmt.Printf("[WARN] Config file not found: %s\n", configFile)
		} else {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				fmt.Printf("[WARN] Failed to parse config file: %v\n", err)
			} else {
				fmt.Printf("[INFO] Loaded config: %s\n", configFile)
			}
		}
	}

	// 설정 파일 값을 기본값으로 사용, CLI 플래그로 오버라이드
	finalTitle := cfg.Document.Title
	if *title != "" {
		finalTitle = *title
	}
	if finalTitle == "" {
		finalTitle = cfg.ProjectName
	}
	if finalTitle == "" {
		finalTitle = "Document"
	}

	finalSubtitle := cfg.Document.Subtitle
	if *subtitle != "" {
		finalSubtitle = *subtitle
	}

	// Author: document.author 우선, 없으면 organization 사용
	finalAuthor := cfg.Document.Author
	if finalAuthor == "" {
		finalAuthor = cfg.Organization
	}
	if *author != "" {
		finalAuthor = *author
	}

	finalHeader := cfg.Document.Header
	if *header != "" {
		finalHeader = *header
	}

	finalFooter := cfg.Document.Footer
	if *footer != "" {
		finalFooter = *footer
	}

	finalCopyright := cfg.Copyright
	if finalCopyright == "" {
		finalCopyright = cfg.Organization
	}

	finalVersion := *version
	if finalVersion == "" {
		finalVersion = "1.0.0"
	}

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

	// Check if input is file or directory
	info, err := os.Stat(*inputDir)
	if err != nil {
		fmt.Printf("[ERROR] Input path not found: %s\n", *inputDir)
		os.Exit(1)
	}

	var files []string
	if info.IsDir() {
		sidebarPath := filepath.Join(*inputDir, "_sidebar.md")
		files, err = parseSidebar(sidebarPath, *inputDir)
		if err != nil {
			fmt.Printf("[WARN] Could not parse sidebar, scanning directory: %v\n", err)
			files, _ = scanMarkdownFiles(*inputDir)
		}
	} else {
		files = []string{*inputDir}
	}

	fmt.Printf("[INFO] Found %d markdown files\n", len(files))

	// Parse and convert each markdown file
	var sections []ManualSection
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Footnote,
			extension.DefinitionList,
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	for _, file := range files {
		// Skip README.md if we have multiple files (it's usually just a web landing page)
		if len(files) > 1 && strings.EqualFold(filepath.Base(file), "readme.md") {
			fmt.Printf("[INFO] Skipping %s (Web landing page)\n", filepath.Base(file))
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("[WARN] Could not read %s: %v\n", file, err)
			continue
		}

		// Pre-process: Handle Docsify syntax
		stringContent := string(content)
		stringContent = preprocessAlerts(stringContent)
		stringContent = preprocessHighlight(stringContent)
		stringContent = preprocessEmoji(stringContent)

		var buf bytes.Buffer
		if err := md.Convert([]byte(stringContent), &buf); err != nil {
			fmt.Printf("[WARN] Could not convert %s: %v\n", file, err)
			continue
		}

		// Post-process: Convert mermaid code blocks
		htmlContent := buf.String()
		htmlContent = convertMermaidBlocks(htmlContent)
		htmlContent = postProcessAlerts(htmlContent)

		if *embedImgs {
			htmlContent = embedImages(htmlContent, file)
			htmlContent = embedStylesheets(htmlContent, file)
		}

		htmlContent = processUIComponents(htmlContent, file)
		htmlContent = rewriteAssetPaths(htmlContent)
		if *pdfMode {
			htmlContent = rewriteInternalLinks(htmlContent)
		}

		// Extract title and level from first heading
		titleText, level := extractTitle(string(content))
		id := generateID(file)

		// Extract sub-headings for hierarchical TOC
		subHeadings := extractSubHeadings(htmlContent)

		// Merge content if level is 2 (H2) and there is a previous section
		if level == 2 && len(sections) > 0 {
			lastIdx := len(sections) - 1
			// Append content to previous section with a separator line
			sections[lastIdx].Content += fmt.Sprintf("\n<div id=\"%s\"></div>\n%s", id, htmlContent)
			// Also merge sub-headings
			sections[lastIdx].SubHeadings = append(sections[lastIdx].SubHeadings, subHeadings...)
			fmt.Printf("[INFO] Merged %s into previous section '%s'\n", file, sections[lastIdx].Title)
			continue
		}

		sections = append(sections, ManualSection{
			Title:       titleText,
			ID:          id,
			Content:     htmlContent,
			Level:       level,
			SubHeadings: subHeadings,
		})
	}

	// 섹션 목록을 JSON으로 출력 (2-Pass의 1단계)
	if *sectionsJSONOut != "" {
		jsonData, err := json.MarshalIndent(sections, "", "  ")
		if err != nil {
			fmt.Printf("[ERROR] Failed to marshal sections: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*sectionsJSONOut, jsonData, 0644); err != nil {
			fmt.Printf("[ERROR] Failed to write sections JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[INFO] Sections JSON saved: %s\n", *sectionsJSONOut)
	}

	// 페이지 번호 JSON 읽기 (2-Pass의 2단계)
	if *pagesJSONIn != "" {
		type PageInfo struct {
			ID   string `json:"id"`
			Page int    `json:"page"`
		}
		type PagesData struct {
			Sections []PageInfo `json:"sections"`
		}

		jsonData, err := os.ReadFile(*pagesJSONIn)
		if err != nil {
			fmt.Printf("[WARN] Could not read pages JSON: %v\n", err)
		} else {
			var pagesData PagesData
			if err := json.Unmarshal(jsonData, &pagesData); err != nil {
				fmt.Printf("[WARN] Could not parse pages JSON: %v\n", err)
			} else {
				// 페이지 번호를 섹션에 적용
				pageMap := make(map[string]int)
				for _, p := range pagesData.Sections {
					pageMap[p.ID] = p.Page
				}
				for i := range sections {
					if page, ok := pageMap[sections[i].ID]; ok {
						sections[i].PageNumber = page
						fmt.Printf("[INFO] Section '%s' -> page %d\n", sections[i].Title, page)
					}

					// 서브헤딩에도 페이지 번호 적용
					for j := range sections[i].SubHeadings {
						if page, ok := pageMap[sections[i].SubHeadings[j].ID]; ok {
							sections[i].SubHeadings[j].PageNumber = page
							fmt.Printf("[INFO]   SubHeading '%s' -> page %d\n", sections[i].SubHeadings[j].Title, page)
						}
					}
				}
			}
		}
	}

	// Generate HTML
	htmlContent, err := generateHTML(finalTitle, finalSubtitle, finalVersion, finalAuthor, finalHeader, finalFooter, finalCopyright, templateName, sections)
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
	// Parse markdown links: [text](/path/to/file.md) or [text](file.md)
	// Group 1: Text
	// Group 2: Optional path prefix (slash)
	// Group 3: Filename/Path
	re := regexp.MustCompile(`\[([^\]]+)\]\(((?:/)?)([^)]+\.md)\)`)
	matches := re.FindAllStringSubmatch(string(content), -1)

	var files []string
	for _, match := range matches {
		if len(match) >= 4 {
			// match[3] is the path without the optional leading slash
			path := match[3]
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

func extractTitle(content string) (string, int) {
	lines := strings.Split(content, "\n")
	var secondChoice string
	inCodeBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# "), 1
		}
		if secondChoice == "" && strings.HasPrefix(line, "## ") {
			secondChoice = strings.TrimPrefix(line, "## ")
		}
	}
	if secondChoice != "" {
		return secondChoice, 2
	}
	return "Untitled", 0
}

func generateID(filePath string) string {
	base := filepath.Base(filePath)
	base = strings.TrimSuffix(base, ".md")
	base = strings.ReplaceAll(base, " ", "-")
	return strings.ToLower(base)
}

// extractSubHeadings extracts H2 headings from HTML content for hierarchical TOC
// H3 is excluded (2-level TOC), and Q. prefixed FAQ items are also excluded
func extractSubHeadings(htmlContent string) []SubHeading {
	var subHeadings []SubHeading

	// Pattern to match <h2 id="...">...</h2> (Allows nested tags like <code>)
	re := regexp.MustCompile(`<h2\s+id="([^"]+)"[^>]*>(.*?)</h2>`)
	matches := re.FindAllStringSubmatch(htmlContent, -1)

	// HTML 태그 제거를 위한 정규식
	stripTags := regexp.MustCompile(`<[^>]*>`)

	for _, match := range matches {
		if len(match) >= 3 {
			rawTitle := match[2]
			// TOC에는 태그가 제거된 순수 텍스트만 표시
			title := strings.TrimSpace(stripTags.ReplaceAllString(rawTitle, ""))
			// HTML 엔티티(&amp; 등)를 일반 문자로 변환 (PDF 분석 매칭용)
			title = gohtml.UnescapeString(title)

			// "Q."로 시작하는 FAQ 항목은 목차에서 제외
			if strings.HasPrefix(title, "Q.") || strings.HasPrefix(title, "Q ") {
				continue
			}
			subHeadings = append(subHeadings, SubHeading{
				Title: title,
				ID:    match[1],
				Level: 2,
			})
		}
	}

	return subHeadings
}

// convertMermaidBlocks converts <pre><code class="language-mermaid"> to <div class="mermaid">
func convertMermaidBlocks(html string) string {
	// Pattern: <pre><code class="language-mermaid">...</code></pre>
	re := regexp.MustCompile(`(?s)<pre><code class="language-mermaid">(.*?)</code></pre>`)
	return re.ReplaceAllString(html, `<div class="mermaid">$1</div>`)
}

// preprocessAlerts는 Docsify(!>, ?>), Docusaurus(:::type) 구문을 GFM Alert 형식으로 변환합니다.
func preprocessAlerts(content string) string {
	lines := strings.Split(content, "\n")
	var newLines []string
	inAlert := false
	inDocusaurus := false

	// Docusaurus 타입을 GFM 타입으로 매핑
	docusaurusMap := map[string]string{
		"note":    "NOTE",
		"tip":     "TIP",
		"info":    "NOTE",
		"warning": "WARNING",
		"danger":  "CAUTION",
		"caution": "CAUTION",
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Docusaurus 구문 시작: :::note, :::tip[제목] 등
		if strings.HasPrefix(trimmed, ":::") && !strings.HasSuffix(trimmed, ":::") {
			// :::type 또는 :::type[title] 파싱
			rest := strings.TrimPrefix(trimmed, ":::")
			typePart := rest
			title := ""

			// [title] 추출
			if idx := strings.Index(rest, "["); idx != -1 {
				typePart = rest[:idx]
				if endIdx := strings.Index(rest, "]"); endIdx != -1 {
					title = rest[idx+1 : endIdx]
				}
			}

			typePart = strings.ToLower(strings.TrimSpace(typePart))
			if gfmType, ok := docusaurusMap[typePart]; ok {
				inDocusaurus = true
				// 첫 줄 생성: > [!TYPE] 또는 > [!TYPE] **제목**
				if title != "" {
					newLines = append(newLines, fmt.Sprintf("> [!%s] **%s**", gfmType, title))
				} else {
					newLines = append(newLines, fmt.Sprintf("> [!%s]", gfmType))
				}
				continue
			}
		}

		// Docusaurus 구문 종료: :::
		if inDocusaurus && trimmed == ":::" {
			inDocusaurus = false
			newLines = append(newLines, "")
			continue
		}

		// Docusaurus 블록 내부
		if inDocusaurus {
			if trimmed == "" {
				newLines = append(newLines, ">")
			} else {
				newLines = append(newLines, "> "+line)
			}
			continue
		}

		// Docsify 구문: !> (Important), ?> (Tip)
		isImportant := strings.HasPrefix(trimmed, "!> ")
		isTip := strings.HasPrefix(trimmed, "?> ")

		if isImportant || isTip {
			inAlert = true
			prefix := "> [!IMPORTANT] "
			clean := strings.TrimPrefix(trimmed, "!> ")
			if isTip {
				prefix = "> [!TIP] "
				clean = strings.TrimPrefix(trimmed, "?> ")
			}
			newLines = append(newLines, prefix+clean)
		} else if inAlert {
			if trimmed == "" {
				inAlert = false
				newLines = append(newLines, "")
			} else {
				// Continuation of alert - treated as blockquote
				newLines = append(newLines, "> "+line)
			}
		} else {
			newLines = append(newLines, line)
		}
	}
	return strings.Join(newLines, "\n")
}

func postProcessAlerts(htmlContent string) string {
	// GFM Alert 타입별 설정: [타입] -> (CSS 클래스, 아이콘)
	// NOTE, TIP, IMPORTANT, WARNING, CAUTION
	alertTypes := []struct {
		Tag   string
		Class string
		Icon  string
	}{
		{"NOTE", "alert-note", "fa-info-circle"},
		{"TIP", "alert-tip", "fa-lightbulb"},
		{"IMPORTANT", "alert-important", "fa-exclamation-circle"},
		{"WARNING", "alert-warning", "fa-triangle-exclamation"},
		{"CAUTION", "alert-caution", "fa-radiation"},
	}

	// 각 Alert 타입에 대해 blockquote를 alert div로 변환
	for _, at := range alertTypes {
		pattern := fmt.Sprintf(`(?s)<blockquote>\s*<p>\s*\[!%s\]\s*(.*?)</p>(\s*.*?)</blockquote>`, at.Tag)
		re := regexp.MustCompile(pattern)
		replacement := fmt.Sprintf(`<div class="alert %s"><div class="alert-icon"><i class="fas %s"></i></div><div class="alert-content"><p>$1</p>$2</div></div>`, at.Class, at.Icon)
		htmlContent = re.ReplaceAllString(htmlContent, replacement)
	}

	// 2차 변환: <p><strong>제목</strong>: 내용</p> 패턴을 제목/본문으로 분리
	// 예: <p><strong>알림</strong>: 다른 곳에서...</p> → <div class="alert-title">알림</div><p class="alert-body">다른 곳에서...</p>
	reTitleBody := regexp.MustCompile(`<p><strong>([^<]+)</strong>\s*:\s*(.+?)</p>`)
	htmlContent = reTitleBody.ReplaceAllString(htmlContent, `<div class="alert-title">$1</div><p class="alert-body">$2</p>`)

	return htmlContent
}

// preprocessHighlight는 ==텍스트== 구문을 <mark>텍스트</mark>로 변환합니다.
func preprocessHighlight(content string) string {
	re := regexp.MustCompile(`==([^=]+)==`)
	return re.ReplaceAllString(content, "<mark>$1</mark>")
}

// preprocessEmoji는 :emoji: 단축코드를 Unicode 이모지로 변환합니다.
func preprocessEmoji(content string) string {
	// 주요 이모지 매핑 테이블
	emojiMap := map[string]string{
		// 일반
		":+1:": "👍", ":-1:": "👎", ":heart:": "❤️", ":star:": "⭐",
		":fire:": "🔥", ":rocket:": "🚀", ":sparkles:": "✨", ":eyes:": "👀",
		":clap:": "👏", ":muscle:": "💪", ":pray:": "🙏", ":wave:": "👋",
		// 상태/알림
		":warning:": "⚠️", ":x:": "❌", ":white_check_mark:": "✅", ":heavy_check_mark:": "✔️",
		":question:": "❓", ":exclamation:": "❗", ":bangbang:": "‼️",
		":info:": "ℹ️", ":bulb:": "💡", ":memo:": "📝", ":book:": "📖",
		// 감정
		":smile:": "😊", ":grin:": "😁", ":joy:": "😂", ":thinking:": "🤔",
		":sunglasses:": "😎", ":sob:": "😭", ":confused:": "😕", ":rage:": "😡",
		// 개발
		":bug:": "🐛", ":wrench:": "🔧", ":hammer:": "🔨", ":gear:": "⚙️",
		":lock:": "🔒", ":key:": "🔑", ":package:": "📦", ":link:": "🔗",
		":zap:": "⚡", ":construction:": "🚧", ":recycle:": "♻️", ":trash:": "🗑️",
		// 화살표
		":arrow_right:": "➡️", ":arrow_left:": "⬅️", ":arrow_up:": "⬆️", ":arrow_down:": "⬇️",
		":point_right:": "👉", ":point_left:": "👈", ":point_up:": "👆", ":point_down:": "👇",
	}

	re := regexp.MustCompile(`:([a-z0-9_+-]+):`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		if emoji, ok := emojiMap[match]; ok {
			return emoji
		}
		return match // 매핑되지 않은 이모지는 그대로 유지
	})
}

func embedImages(htmlContent, mdFilePath string) string {
	// Find all img tags: <img src="..." ...>
	// We use regex for simplicity. Be careful with complex HTML.
	// Capture group 1: src value
	re := regexp.MustCompile(`<img[^>]+src="([^"]+)"[^>]*>`)

	return re.ReplaceAllStringFunc(htmlContent, func(imgTag string) string {
		// Extract src
		subMatch := re.FindStringSubmatch(imgTag)
		if len(subMatch) < 2 {
			return imgTag
		}
		src := subMatch[1]

		// Skip remote URLs or existing data URIs
		if strings.HasPrefix(src, "http") || strings.HasPrefix(src, "data:") {
			return imgTag
		}

		// Calculate absolute path of image
		// mdFilePath is absolute path to the markdown file
		dir := filepath.Dir(mdFilePath)
		imgPath := filepath.Join(dir, src)

		// Read file
		data, err := os.ReadFile(imgPath)
		if err != nil {
			fmt.Printf("[WARN] Failed to read image for embedding: %s (%v)\n", imgPath, err)
			return imgTag
		}

		// Detect MIME type
		mimeType := mime.TypeByExtension(filepath.Ext(imgPath))
		if mimeType == "" {
			// Fallback detection
			mimeType = http.DetectContentType(data)
		}

		// Base64 encode
		encoded := base64.StdEncoding.EncodeToString(data)
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)

		// Replace src in the tag
		return strings.Replace(imgTag, src, dataURI, 1)
	})
}

func embedStylesheets(htmlContent, mdFilePath string) string {
	// Find all link tags for stylesheets: <link rel="stylesheet" href="...">
	re := regexp.MustCompile(`<link[^>]+rel="stylesheet"[^>]+href="([^"]+)"[^>]*>`)

	return re.ReplaceAllStringFunc(htmlContent, func(linkTag string) string {
		// Extract href
		subMatch := re.FindStringSubmatch(linkTag)
		if len(subMatch) < 2 {
			return linkTag
		}
		href := subMatch[1]

		// Skip remote URLs
		if strings.HasPrefix(href, "http") {
			return linkTag
		}

		// Calculate absolute path of CSS
		dir := filepath.Dir(mdFilePath)
		cssPath := filepath.Join(dir, href)

		// Read file
		data, err := os.ReadFile(cssPath)
		if err != nil {
			fmt.Printf("[WARN] Failed to read CSS for embedding: %s (%v)\n", cssPath, err)
			return linkTag
		}

		// Wrap in style block
		return fmt.Sprintf("<style>\n%s\n</style>", string(data))
	})
}

func processUIComponents(htmlContent, mdFilePath string) string {
	// Find all UI component markers: <!-- @ui:component-name -->
	re := regexp.MustCompile(`<!--\s*@ui:([a-zA-Z0-9_-]+)\s*-->`)

	return re.ReplaceAllStringFunc(htmlContent, func(marker string) string {
		// Extract component name
		subMatch := re.FindStringSubmatch(marker)
		if len(subMatch) < 2 {
			return marker
		}
		componentName := subMatch[1]

		// Components are expected to be in assets/ui/ relative to manual root
		dir := filepath.Dir(mdFilePath)
		var assetsDir string

		// Look up for 'assets' directory
		curr := dir
		for i := 0; i < 5; i++ { // limit search depth
			testPath := filepath.Join(curr, "assets", "ui", componentName+".html")
			if _, err := os.Stat(testPath); err == nil {
				assetsDir = testPath
				break
			}
			parent := filepath.Dir(curr)
			if parent == curr {
				break
			}
			curr = parent
		}

		if assetsDir == "" {
			fmt.Printf("[WARN] UI Component not found: %s\n", componentName)
			return marker
		}

		// Read component file
		data, err := os.ReadFile(assetsDir)
		if err != nil {
			fmt.Printf("[WARN] Failed to read UI component file: %s (%v)\n", assetsDir, err)
			return marker
		}

		return string(data)
	})
}

// rewriteAssetPaths normalizes asset paths to be relative to the output HTML
// This runs AFTER embedding, so it only affects images that failed to embed or were skipped.
func rewriteAssetPaths(html string) string {
	// Pattern: src="../../assets/..." -> src="assets/..."
	re := regexp.MustCompile(`src="(?:\.\./)+assets/`)
	return re.ReplaceAllString(html, `src="assets/`)
}

// rewriteInternalLinks converts relative .md links to internal PDF anchors
// Example: href="./02_service_mgmt.md#section" → href="#section" (normalized)
// Example: href="./01_basics.md" → href="#31---" (based on heading ID)
// External links (http/https) are preserved
func rewriteInternalLinks(htmlContent string) string {
	// Pattern: href="./path/to/file.md#anchor" or href="/path/file.md#anchor"
	re := regexp.MustCompile(`href="\.?/?([^"]*\.md)(#[^"]*)?"`)

	return re.ReplaceAllStringFunc(htmlContent, func(match string) string {
		subMatch := re.FindStringSubmatch(match)
		if len(subMatch) < 2 {
			return match
		}

		// Extract anchor part (#...)
		anchor := ""
		if len(subMatch) >= 3 && subMatch[2] != "" {
			anchor = subMatch[2]
		}

		if anchor != "" {
			// Has explicit anchor - normalize it to match goldmark's heading ID format
			normalized := normalizeAnchor(anchor)
			return fmt.Sprintf(`href="%s"`, normalized)
		}

		// No anchor - extract filename and create section-based anchor
		mdPath := subMatch[1]
		baseName := filepath.Base(mdPath)
		baseName = strings.TrimSuffix(baseName, ".md")

		// Map known files to their heading IDs (goldmark generates these)
		// Note: Goldmark strips Korean characters, leaving only numbers and English
		headingMap := map[string]string{
			"01_basics":         "31---",
			"02_service_mgmt":   "32---service",
			"03_monitoring":     "33----monitoring",
			"04_backup_restore": "34----backuprestore",
			"05_environment":    "35---env",
			"06_security":       "36----security-analysis--remediation",
		}

		if anchor, exists := headingMap[baseName]; exists {
			return fmt.Sprintf(`href="#%s"`, anchor)
		}

		// Fallback: use lowercase filename as anchor
		return fmt.Sprintf(`href="#%s"`, strings.ToLower(baseName))
	})
}

// normalizeAnchor converts anchor text to goldmark-style heading ID
// Goldmark removes non-ASCII characters (like Korean) and converts to lowercase
// Example: #322-서비스-로그-로테이션-상태-확인-및-설정-logrotate → #322--------logrotate
func normalizeAnchor(anchor string) string {
	// URL decode first (handle %EC%84%9C%EB%B9%84%EC%8A%A4 etc.)
	decoded, err := url.QueryUnescape(anchor)
	if err != nil {
		decoded = anchor
	}

	// Remove the leading #
	if strings.HasPrefix(decoded, "#") {
		decoded = decoded[1:]
	}

	// Keep only ASCII letters, numbers, and hyphens (goldmark behavior)
	var result strings.Builder
	for _, r := range decoded {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		} else if r == ' ' || r == '_' {
			result.WriteRune('-')
		}
		// Non-ASCII characters (Korean, etc.) are dropped
	}

	// Lowercase and clean up multiple hyphens
	normalized := strings.ToLower(result.String())

	return "#" + normalized
}

//go:embed templates/*.html
var templateFS embed.FS

func generateHTML(title, subtitle, version, author, header, footer, copyright, templateName string, sections []ManualSection) (string, error) {
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
			if len(s) < start {
				return s
			}
			if len(s) < end {
				return s[start:]
			}
			return s[start:end]
		},
	}

	t, err := template.New("manual").Funcs(funcMap).Parse(string(tmplData))
	if err != nil {
		return "", err
	}

	// 기본값 설정: 헤더가 비어있으면 "Title - Subtitle" 형식 사용
	if header == "" {
		if subtitle != "" {
			header = title + " - " + subtitle
		} else {
			header = title
		}
	}
	// 푸터가 비어있으면 Author 사용
	if footer == "" {
		footer = author
	}

	var buf bytes.Buffer
	data := ManualConfig{
		Title:     title,
		Subtitle:  subtitle,
		Version:   version,
		Date:      findDate(),
		Author:    author,
		Header:    header,
		Footer:    footer,
		Copyright: copyright,
		Sections:  sections,
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
