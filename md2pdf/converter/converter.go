// Package converter converts Markdown files to HTML documents.
// Extracted from md2html_v2 project.
package converter

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
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

// SubHeading represents a sub-heading within a section (H2, H3, etc.)
type SubHeading struct {
	Title      string `json:"title"`
	ID         string `json:"id"`
	Level      int    `json:"level"`
	PageNumber int    `json:"page,omitempty"`
}

// Section represents a section of the manual
type Section struct {
	Title       string       `json:"title"`
	ID          string       `json:"id"`
	Content     string       `json:"-"`
	Level       int          `json:"level"`
	SubHeadings []SubHeading `json:"subheadings,omitempty"`
	PageNumber  int          `json:"page,omitempty"`
}

// ManualConfig defines template data
type ManualConfig struct {
	Title     string
	Subtitle  string
	Version   string
	Date      string
	Author    string
	Header    string
	Footer    string
	Copyright string
	Sections  []Section
}

// Options for HTML conversion
type Options struct {
	InputDir     string
	OutputFile   string
	ConfigFile   string
	Title        string
	Subtitle     string
	Version      string
	Author       string
	Header       string
	Footer       string
	Template     string
	EmbedImages  bool
	PDFMode      bool
	SectionsJSON string // Output sections JSON path
	PagesJSON    string // Input pages JSON path
}

//go:embed templates/*.html
var templateFS embed.FS

// ConvertToHTML converts markdown files to a single HTML document.
// Returns the list of sections for PDF analysis.
func ConvertToHTML(opts Options) ([]Section, error) {
	// Load config file
	var cfg AuthorsConfig
	if opts.ConfigFile != "" {
		data, err := os.ReadFile(opts.ConfigFile)
		if err != nil {
			fmt.Printf("[WARN] Config file not found: %s\n", opts.ConfigFile)
		} else {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				fmt.Printf("[WARN] Failed to parse config file: %v\n", err)
			} else {
				fmt.Printf("[INFO] Loaded config: %s\n", opts.ConfigFile)
			}
		}
	}

	// Resolve final values (CLI overrides config)
	finalTitle := resolveValue(opts.Title, cfg.Document.Title, cfg.ProjectName, "Document")
	finalSubtitle := resolveValue(opts.Subtitle, cfg.Document.Subtitle, "", "")
	finalAuthor := resolveValue(opts.Author, cfg.Document.Author, cfg.Organization, "")
	finalHeader := resolveValue(opts.Header, cfg.Document.Header, "", "")
	finalFooter := resolveValue(opts.Footer, cfg.Document.Footer, "", "")
	finalCopyright := resolveValue("", cfg.Copyright, cfg.Organization, "")
	finalVersion := opts.Version
	if finalVersion == "" {
		finalVersion = "1.0.0"
	}
	templateName := opts.Template
	if templateName == "" {
		templateName = "report"
	}

	// Discover markdown files
	info, err := os.Stat(opts.InputDir)
	if err != nil {
		return nil, fmt.Errorf("input path not found: %s", opts.InputDir)
	}

	var files []string
	if info.IsDir() {
		sidebarPath := filepath.Join(opts.InputDir, "_sidebar.md")
		files, err = parseSidebar(sidebarPath, opts.InputDir)
		if err != nil {
			fmt.Printf("[WARN] Could not parse sidebar, scanning directory: %v\n", err)
			files, _ = scanMarkdownFiles(opts.InputDir)
		}
	} else {
		files = []string{opts.InputDir}
	}

	fmt.Printf("[INFO] Found %d markdown files\n", len(files))

	// Goldmark setup
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

	// Convert each file
	var sections []Section
	for _, file := range files {
		if len(files) > 1 && strings.EqualFold(filepath.Base(file), "readme.md") {
			fmt.Printf("[INFO] Skipping %s (Web landing page)\n", filepath.Base(file))
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("[WARN] Could not read %s: %v\n", file, err)
			continue
		}

		// Pre-process
		stringContent := string(content)
		stringContent = preprocessAlerts(stringContent)
		stringContent = preprocessHighlight(stringContent)
		stringContent = preprocessEmoji(stringContent)

		var buf bytes.Buffer
		if err := md.Convert([]byte(stringContent), &buf); err != nil {
			fmt.Printf("[WARN] Could not convert %s: %v\n", file, err)
			continue
		}

		// Post-process
		htmlContent := buf.String()
		htmlContent = convertMermaidBlocks(htmlContent)
		htmlContent = postProcessAlerts(htmlContent)

		if opts.EmbedImages {
			htmlContent = embedImages(htmlContent, file)
			htmlContent = embedStylesheets(htmlContent, file)
		}

		htmlContent = processUIComponents(htmlContent, file)
		htmlContent = rewriteAssetPaths(htmlContent)
		if opts.PDFMode {
			htmlContent = rewriteInternalLinks(htmlContent)
		}

		titleText, level := extractTitle(string(content))
		id := generateID(file)
		subHeadings := extractSubHeadings(htmlContent)

		// Merge H2 sections into previous
		if level == 2 && len(sections) > 0 {
			lastIdx := len(sections) - 1
			sections[lastIdx].Content += fmt.Sprintf("\n<div id=\"%s\"></div>\n%s", id, htmlContent)
			sections[lastIdx].SubHeadings = append(sections[lastIdx].SubHeadings, subHeadings...)
			fmt.Printf("[INFO] Merged %s into previous section '%s'\n", file, sections[lastIdx].Title)
			continue
		}

		sections = append(sections, Section{
			Title:       titleText,
			ID:          id,
			Content:     htmlContent,
			Level:       level,
			SubHeadings: subHeadings,
		})
	}

	// Output sections JSON (for 2-Pass)
	if opts.SectionsJSON != "" {
		jsonData, err := json.MarshalIndent(sections, "", "  ")
		if err != nil {
			return sections, fmt.Errorf("failed to marshal sections: %w", err)
		}
		if err := os.WriteFile(opts.SectionsJSON, jsonData, 0644); err != nil {
			return sections, fmt.Errorf("failed to write sections JSON: %w", err)
		}
		fmt.Printf("[INFO] Sections JSON saved: %s\n", opts.SectionsJSON)
	}

	// Load page numbers (for 2-Pass, Pass 2)
	if opts.PagesJSON != "" {
		applyPageNumbers(opts.PagesJSON, sections)
	}

	// Generate HTML
	htmlContent, err := generateHTML(finalTitle, finalSubtitle, finalVersion, finalAuthor, finalHeader, finalFooter, finalCopyright, templateName, sections)
	if err != nil {
		return sections, fmt.Errorf("failed to generate HTML: %w", err)
	}

	if err := os.WriteFile(opts.OutputFile, []byte(htmlContent), 0644); err != nil {
		return sections, fmt.Errorf("failed to write output: %w", err)
	}

	fmt.Printf("[SUCCESS] Generated HTML: %s\n", opts.OutputFile)
	return sections, nil
}

// --- Helper functions (extracted from md2html_v2) ---

func resolveValue(cliValue, configValue, fallback, defaultVal string) string {
	if cliValue != "" {
		return cliValue
	}
	if configValue != "" {
		return configValue
	}
	if fallback != "" {
		return fallback
	}
	return defaultVal
}

func applyPageNumbers(pagesJSONPath string, sections []Section) {
	type PageInfo struct {
		ID   string `json:"id"`
		Page int    `json:"page"`
	}
	type PagesData struct {
		Sections []PageInfo `json:"sections"`
	}

	jsonData, err := os.ReadFile(pagesJSONPath)
	if err != nil {
		fmt.Printf("[WARN] Could not read pages JSON: %v\n", err)
		return
	}

	var pagesData PagesData
	if err := json.Unmarshal(jsonData, &pagesData); err != nil {
		fmt.Printf("[WARN] Could not parse pages JSON: %v\n", err)
		return
	}

	pageMap := make(map[string]int)
	for _, p := range pagesData.Sections {
		pageMap[p.ID] = p.Page
	}
	for i := range sections {
		if page, ok := pageMap[sections[i].ID]; ok {
			sections[i].PageNumber = page
			fmt.Printf("[INFO] Section '%s' -> page %d\n", sections[i].Title, page)
		}
		for j := range sections[i].SubHeadings {
			if page, ok := pageMap[sections[i].SubHeadings[j].ID]; ok {
				sections[i].SubHeadings[j].PageNumber = page
				fmt.Printf("[INFO]   SubHeading '%s' -> page %d\n", sections[i].SubHeadings[j].Title, page)
			}
		}
	}
}

func parseSidebar(sidebarPath, baseDir string) ([]string, error) {
	content, err := os.ReadFile(sidebarPath)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`\[([^\]]+)\]\(((?:/)?)([^)]+\.md)\)`)
	matches := re.FindAllStringSubmatch(string(content), -1)

	var files []string
	for _, match := range matches {
		if len(match) >= 4 {
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

func extractSubHeadings(htmlContent string) []SubHeading {
	var subHeadings []SubHeading
	re := regexp.MustCompile(`<h2\s+id="([^"]+)"[^>]*>(.*?)</h2>`)
	matches := re.FindAllStringSubmatch(htmlContent, -1)
	stripTags := regexp.MustCompile(`<[^>]*>`)

	for _, match := range matches {
		if len(match) >= 3 {
			rawTitle := match[2]
			title := strings.TrimSpace(stripTags.ReplaceAllString(rawTitle, ""))
			title = gohtml.UnescapeString(title)
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

func convertMermaidBlocks(h string) string {
	re := regexp.MustCompile(`(?s)<pre><code class="language-mermaid">(.*?)</code></pre>`)
	return re.ReplaceAllString(h, `<div class="mermaid">$1</div>`)
}

func preprocessAlerts(content string) string {
	lines := strings.Split(content, "\n")
	var newLines []string
	inAlert := false
	inDocusaurus := false

	docusaurusMap := map[string]string{
		"note": "NOTE", "tip": "TIP", "info": "NOTE",
		"warning": "WARNING", "danger": "CAUTION", "caution": "CAUTION",
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, ":::") && !strings.HasSuffix(trimmed, ":::") {
			rest := strings.TrimPrefix(trimmed, ":::")
			typePart := rest
			title := ""
			if idx := strings.Index(rest, "["); idx != -1 {
				typePart = rest[:idx]
				if endIdx := strings.Index(rest, "]"); endIdx != -1 {
					title = rest[idx+1 : endIdx]
				}
			}
			typePart = strings.ToLower(strings.TrimSpace(typePart))
			if gfmType, ok := docusaurusMap[typePart]; ok {
				inDocusaurus = true
				if title != "" {
					newLines = append(newLines, fmt.Sprintf("> [!%s] **%s**", gfmType, title))
				} else {
					newLines = append(newLines, fmt.Sprintf("> [!%s]", gfmType))
				}
				continue
			}
		}

		if inDocusaurus && trimmed == ":::" {
			inDocusaurus = false
			newLines = append(newLines, "")
			continue
		}

		if inDocusaurus {
			if trimmed == "" {
				newLines = append(newLines, ">")
			} else {
				newLines = append(newLines, "> "+line)
			}
			continue
		}

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
				newLines = append(newLines, "> "+line)
			}
		} else {
			newLines = append(newLines, line)
		}
	}
	return strings.Join(newLines, "\n")
}

func postProcessAlerts(htmlContent string) string {
	alertTypes := []struct {
		Tag, Class, Icon string
	}{
		{"NOTE", "alert-note", "fa-info-circle"},
		{"TIP", "alert-tip", "fa-lightbulb"},
		{"IMPORTANT", "alert-important", "fa-exclamation-circle"},
		{"WARNING", "alert-warning", "fa-triangle-exclamation"},
		{"CAUTION", "alert-caution", "fa-radiation"},
	}

	for _, at := range alertTypes {
		pattern := fmt.Sprintf(`(?s)<blockquote>\s*<p>\s*\[!%s\]\s*(.*?)</p>(\s*.*?)</blockquote>`, at.Tag)
		re := regexp.MustCompile(pattern)
		replacement := fmt.Sprintf(`<div class="alert %s"><div class="alert-icon"><i class="fas %s"></i></div><div class="alert-content"><p>$1</p>$2</div></div>`, at.Class, at.Icon)
		htmlContent = re.ReplaceAllString(htmlContent, replacement)
	}

	reTitleBody := regexp.MustCompile(`<p><strong>([^<]+)</strong>\s*:\s*(.+?)</p>`)
	htmlContent = reTitleBody.ReplaceAllString(htmlContent, `<div class="alert-title">$1</div><p class="alert-body">$2</p>`)

	return htmlContent
}

func preprocessHighlight(content string) string {
	re := regexp.MustCompile(`==([^=]+)==`)
	return re.ReplaceAllString(content, "<mark>$1</mark>")
}

func preprocessEmoji(content string) string {
	emojiMap := map[string]string{
		":+1:": "👍", ":-1:": "👎", ":heart:": "❤️", ":star:": "⭐",
		":fire:": "🔥", ":rocket:": "🚀", ":sparkles:": "✨", ":eyes:": "👀",
		":clap:": "👏", ":muscle:": "💪", ":pray:": "🙏", ":wave:": "👋",
		":warning:": "⚠️", ":x:": "❌", ":white_check_mark:": "✅", ":heavy_check_mark:": "✔️",
		":question:": "❓", ":exclamation:": "❗", ":bangbang:": "‼️",
		":info:": "ℹ️", ":bulb:": "💡", ":memo:": "📝", ":book:": "📖",
		":smile:": "😊", ":grin:": "😁", ":joy:": "😂", ":thinking:": "🤔",
		":sunglasses:": "😎", ":sob:": "😭", ":confused:": "😕", ":rage:": "😡",
		":bug:": "🐛", ":wrench:": "🔧", ":hammer:": "🔨", ":gear:": "⚙️",
		":lock:": "🔒", ":key:": "🔑", ":package:": "📦", ":link:": "🔗",
		":zap:": "⚡", ":construction:": "🚧", ":recycle:": "♻️", ":trash:": "🗑️",
		":arrow_right:": "➡️", ":arrow_left:": "⬅️", ":arrow_up:": "⬆️", ":arrow_down:": "⬇️",
		":point_right:": "👉", ":point_left:": "👈", ":point_up:": "👆", ":point_down:": "👇",
	}

	re := regexp.MustCompile(`:([a-z0-9_+-]+):`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		if emoji, ok := emojiMap[match]; ok {
			return emoji
		}
		return match
	})
}

func embedImages(htmlContent, mdFilePath string) string {
	re := regexp.MustCompile(`<img[^>]+src="([^"]+)"[^>]*>`)
	return re.ReplaceAllStringFunc(htmlContent, func(imgTag string) string {
		subMatch := re.FindStringSubmatch(imgTag)
		if len(subMatch) < 2 {
			return imgTag
		}
		src := subMatch[1]
		if strings.HasPrefix(src, "http") || strings.HasPrefix(src, "data:") {
			return imgTag
		}
		dir := filepath.Dir(mdFilePath)
		imgPath := filepath.Join(dir, src)
		data, err := os.ReadFile(imgPath)
		if err != nil {
			fmt.Printf("[WARN] Failed to read image for embedding: %s (%v)\n", imgPath, err)
			return imgTag
		}
		mimeType := mime.TypeByExtension(filepath.Ext(imgPath))
		if mimeType == "" {
			mimeType = http.DetectContentType(data)
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
		return strings.Replace(imgTag, src, dataURI, 1)
	})
}

func embedStylesheets(htmlContent, mdFilePath string) string {
	re := regexp.MustCompile(`<link[^>]+rel="stylesheet"[^>]+href="([^"]+)"[^>]*>`)
	return re.ReplaceAllStringFunc(htmlContent, func(linkTag string) string {
		subMatch := re.FindStringSubmatch(linkTag)
		if len(subMatch) < 2 {
			return linkTag
		}
		href := subMatch[1]
		if strings.HasPrefix(href, "http") {
			return linkTag
		}
		dir := filepath.Dir(mdFilePath)
		cssPath := filepath.Join(dir, href)
		data, err := os.ReadFile(cssPath)
		if err != nil {
			fmt.Printf("[WARN] Failed to read CSS for embedding: %s (%v)\n", cssPath, err)
			return linkTag
		}
		return fmt.Sprintf("<style>\n%s\n</style>", string(data))
	})
}

func processUIComponents(htmlContent, mdFilePath string) string {
	re := regexp.MustCompile(`<!--\s*@ui:([a-zA-Z0-9_-]+)\s*-->`)
	return re.ReplaceAllStringFunc(htmlContent, func(marker string) string {
		subMatch := re.FindStringSubmatch(marker)
		if len(subMatch) < 2 {
			return marker
		}
		componentName := subMatch[1]
		dir := filepath.Dir(mdFilePath)
		var assetsDir string
		curr := dir
		for i := 0; i < 5; i++ {
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
		data, err := os.ReadFile(assetsDir)
		if err != nil {
			fmt.Printf("[WARN] Failed to read UI component file: %s (%v)\n", assetsDir, err)
			return marker
		}
		return string(data)
	})
}

func rewriteAssetPaths(h string) string {
	re := regexp.MustCompile(`src="(?:\.\./)+assets/`)
	return re.ReplaceAllString(h, `src="assets/`)
}

func rewriteInternalLinks(htmlContent string) string {
	re := regexp.MustCompile(`href="\.?/?([^"]*\.md)(#[^"]*)?"`)
	return re.ReplaceAllStringFunc(htmlContent, func(match string) string {
		subMatch := re.FindStringSubmatch(match)
		if len(subMatch) < 2 {
			return match
		}
		mdPath := subMatch[1]
		anchor := ""
		if len(subMatch) >= 3 {
			anchor = subMatch[2]
		}
		if anchor != "" {
			return fmt.Sprintf(`href="%s"`, normalizeAnchor(anchor))
		}
		id := generateID(mdPath)
		return fmt.Sprintf(`href="#%s"`, id)
	})
}

func normalizeAnchor(anchor string) string {
	decoded, err := url.QueryUnescape(anchor)
	if err != nil {
		decoded = anchor
	}
	if strings.HasPrefix(decoded, "#") {
		decoded = decoded[1:]
	}
	var result strings.Builder
	for _, r := range decoded {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		} else if r == ' ' || r == '_' {
			result.WriteRune('-')
		}
	}
	normalized := strings.ToLower(result.String())
	return "#" + normalized
}

func generateHTML(title, subtitle, version, author, header, footer, copyright, templateName string, sections []Section) (string, error) {
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

	if header == "" {
		if subtitle != "" {
			header = title + " - " + subtitle
		} else {
			header = title
		}
	}
	if footer == "" {
		footer = author
	}

	var buf bytes.Buffer
	data := ManualConfig{
		Title:     title,
		Subtitle:  subtitle,
		Version:   version,
		Date:      time.Now().Format("2006년 01월 02일"),
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
