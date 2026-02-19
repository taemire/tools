// md2pdf - Markdown to PDF converter using gopdf
// generates high-quality PDF documents with headers, footers, and page numbers
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"md2pdf/parser"
	"md2pdf/renderer"

	"gopkg.in/yaml.v3"
)

// Version information
var (
	Version   = "0.1.0"
	BuildDate = "dev"
)

// ProjectConfig matches AUTHORS.yml structure
type ProjectConfig struct {
	Name         string           `yaml:"name"`
	Organization string           `yaml:"organization"`
	Version      string           `yaml:"version"`
	Copyright    string           `yaml:"copyright"`
	Authors      []AuthorInfo     `yaml:"authors"`
	Document     DocumentSettings `yaml:"document"`
}

type AuthorInfo struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
	Role  string `yaml:"role"`
}

type DocumentSettings struct {
	Header         string `yaml:"header"`
	Footer         string `yaml:"footer"`
	ShowPageNumber bool   `yaml:"show_page_number"`
}

func main() {
	// Define flags
	inputPtr := flag.String("input", "", "Input markdown file or directory (required)")
	inputShort := flag.String("i", "", "Input markdown file or directory (shorthand)")

	outputPtr := flag.String("output", "", "Output PDF file path (required)")
	outputShort := flag.String("o", "", "Output PDF file path (shorthand)")

	configPtr := flag.String("config", "", "Path to AUTHORS.yml configuration file")
	configShort := flag.String("c", "", "Path to AUTHORS.yml configuration file (shorthand)")

	themePtr := flag.String("theme", "corporate-blue", "Theme name or path to theme JSON file")
	themeShort := flag.String("t", "", "Theme name or path (shorthand)")

	titlePtr := flag.String("title", "", "Document title (overrides config)")
	versionPtr := flag.String("version", "", "Document version (overrides config)")
	authorPtr := flag.String("author", "", "Document author (overrides config)")

	listThemes := flag.Bool("list-themes", false, "List available themes")
	validateTheme := flag.String("validate-theme", "", "Validate a theme JSON file")
	showVersion := flag.Bool("v", false, "Show version information")
	helpFlag := flag.Bool("h", false, "Show help")

	flag.Parse()

	// Handle shorthand flags
	input := coalesce(*inputPtr, *inputShort)
	output := coalesce(*outputPtr, *outputShort)
	configPath := coalesce(*configPtr, *configShort)
	themeName := coalesce(*themePtr, *themeShort, "corporate-blue")

	// Show version
	if *showVersion {
		fmt.Printf("md2pdf version %s (built: %s)\n", Version, BuildDate)
		os.Exit(0)
	}

	// Show help
	if *helpFlag {
		printUsage()
		os.Exit(0)
	}

	// List themes
	if *listThemes {
		listAvailableThemes()
		os.Exit(0)
	}

	// Validate theme
	if *validateTheme != "" {
		if err := validateThemeFile(*validateTheme); err != nil {
			fmt.Fprintf(os.Stderr, "Theme validation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Theme validation successful!")
		os.Exit(0)
	}

	// Validate required arguments
	if input == "" || output == "" {
		fmt.Fprintln(os.Stderr, "Error: --input and --output are required")
		printUsage()
		os.Exit(1)
	}

	// Load configuration
	var projectConfig ProjectConfig
	if configPath != "" {
		if err := loadYAMLConfig(configPath, &projectConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		}
	}

	// Override with command line arguments
	if *titlePtr != "" {
		projectConfig.Name = *titlePtr
	}
	if *versionPtr != "" {
		projectConfig.Version = *versionPtr
	}
	if *authorPtr != "" && len(projectConfig.Authors) == 0 {
		projectConfig.Authors = []AuthorInfo{{Name: *authorPtr}}
	}

	// Load theme
	theme, err := loadTheme(themeName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading theme: %v\n", err)
		os.Exit(1)
	}

	// Parse markdown
	var doc *parser.MarkdownDocument
	fileInfo, err := os.Stat(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error accessing input: %v\n", err)
		os.Exit(1)
	}

	if fileInfo.IsDir() {
		doc, err = parser.ParseMarkdownDir(input)
	} else {
		doc, err = parser.ParseMarkdownFile(input)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing markdown: %v\n", err)
		os.Exit(1)
	}

	// Prepare document config
	docConfig := &renderer.DocumentConfig{
		Title:        coalesce(projectConfig.Name, doc.Title, "Document"),
		Version:      coalesce(projectConfig.Version, "1.0.0"),
		Author:       getFirstAuthor(projectConfig.Authors),
		Date:         time.Now().Format("2006년 01월 02일"),
		Copyright:    coalesce(projectConfig.Copyright, projectConfig.Organization),
		Organization: coalesce(projectConfig.Organization, ""),
		Header:       coalesce(projectConfig.Document.Header, projectConfig.Name+" 사용자 매뉴얼"),
		Footer:       coalesce(projectConfig.Document.Footer, projectConfig.Copyright),
	}

	// Create renderer
	pdfRenderer := renderer.NewPDFRenderer(theme, docConfig)

	// Load fonts
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	if err := pdfRenderer.LoadFonts(filepath.Join(execDir, "fonts")); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Font loading issue: %v\n", err)
	}

	// Render document
	if err := pdfRenderer.RenderDocument(doc); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering PDF: %v\n", err)
		os.Exit(1)
	}

	// Save PDF
	if err := pdfRenderer.Save(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving PDF: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated: %s\n", output)
}

func printUsage() {
	fmt.Println(`md2pdf - Markdown to PDF converter with theme support

Usage:
  md2pdf -i <input> -o <output> [options]

Options:
  -i, --input <path>       Input markdown file or directory (required)
  -o, --output <path>      Output PDF file path (required)
  -c, --config <path>      Path to AUTHORS.yml configuration file
  -t, --theme <name|path>  Theme name or path to theme JSON (default: corporate-blue)
      --title <title>      Document title (overrides config)
      --version <version>  Document version (overrides config)
      --author <author>    Document author (overrides config)
      --list-themes        List available themes
      --validate-theme     Validate a theme JSON file
  -v                       Show version information
  -h                       Show this help

Examples:
  md2pdf -i docs/manual -o USER_MANUAL.pdf
  md2pdf -i README.md -o output.pdf --theme corporate-blue
  md2pdf -i docs -o manual.pdf -c AUTHORS.yml -t themes/custom.json`)
}

func listAvailableThemes() {
	fmt.Println("Available themes:")
	fmt.Println("  corporate-blue   - 기업용 블루 테마 (기본)")
	fmt.Println("  corporate-dark   - 다크 모드 테마")
	fmt.Println("  minimal-clean    - 미니멀 화이트 테마")
	fmt.Println("  technical-mono   - 기술 문서용 모노톤")
	fmt.Println("  vibrant-modern   - 모던 컬러풀 테마")
}

func validateThemeFile(path string) error {
	_, err := parser.LoadTheme(path)
	return err
}

func loadTheme(nameOrPath string) (*parser.Theme, error) {
	// Check if it's a path
	if strings.HasSuffix(nameOrPath, ".json") || strings.Contains(nameOrPath, "/") || strings.Contains(nameOrPath, "\\") {
		return parser.LoadTheme(nameOrPath)
	}

	// Look for built-in theme
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	themePath := filepath.Join(execDir, "themes", nameOrPath+".json")

	// Fallback to current directory
	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		themePath = filepath.Join("themes", nameOrPath+".json")
	}

	return parser.LoadTheme(themePath)
}

func loadYAMLConfig(path string, config *ProjectConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, config)
}

func getFirstAuthor(authors []AuthorInfo) string {
	if len(authors) > 0 {
		return authors[0].Name
	}
	return ""
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
