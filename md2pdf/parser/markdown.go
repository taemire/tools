// Package parser provides functionality for parsing markdown files
package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MarkdownDocument represents a parsed markdown document
type MarkdownDocument struct {
	Title    string
	Sections []Section
	TOC      []TOCEntry
	Anchors  map[string]int // anchor ID -> section index
}

// Section represents a section of the document
type Section struct {
	Level    int // 1 = h1, 2 = h2, etc.
	Title    string
	AnchorID string
	Content  []Block
}

// Block represents a content block (paragraph, code, table, etc.)
type Block struct {
	Type    BlockType
	Content string
	Items   []string   // for lists
	Rows    [][]string // for tables
	Lang    string     // for code blocks
}

type BlockType int

const (
	BlockParagraph BlockType = iota
	BlockCodeBlock
	BlockList
	BlockNumberedList
	BlockTable
	BlockBlockquote
	BlockImage
)

// TOCEntry represents a table of contents entry
type TOCEntry struct {
	Level    int
	Title    string
	AnchorID string
	Page     int // will be set during rendering
}

// ParseMarkdownFile parses a markdown file and returns a MarkdownDocument
func ParseMarkdownFile(path string) (*MarkdownDocument, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	doc := &MarkdownDocument{
		Sections: make([]Section, 0),
		TOC:      make([]TOCEntry, 0),
		Anchors:  make(map[string]int),
	}

	scanner := bufio.NewScanner(file)
	var currentSection *Section
	var currentBlocks []Block
	var inCodeBlock bool
	var codeBlockContent strings.Builder
	var codeBlockLang string

	headingRegex := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

	lineNumber := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineNumber++

		// Handle code blocks
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End code block
				currentBlocks = append(currentBlocks, Block{
					Type:    BlockCodeBlock,
					Content: codeBlockContent.String(),
					Lang:    codeBlockLang,
				})
				codeBlockContent.Reset()
				codeBlockLang = ""
				inCodeBlock = false
			} else {
				// Start code block
				inCodeBlock = true
				codeBlockLang = strings.TrimPrefix(line, "```")
			}
			continue
		}

		if inCodeBlock {
			if codeBlockContent.Len() > 0 {
				codeBlockContent.WriteString("\n")
			}
			codeBlockContent.WriteString(line)
			continue
		}

		// Check for headings
		if matches := headingRegex.FindStringSubmatch(line); matches != nil {
			// Save previous section
			if currentSection != nil {
				currentSection.Content = currentBlocks
				doc.Sections = append(doc.Sections, *currentSection)
				doc.Anchors[currentSection.AnchorID] = len(doc.Sections) - 1
			}

			level := len(matches[1])
			title := strings.TrimSpace(matches[2])
			anchorID := generateAnchorID(title)

			// Set document title from first h1
			if level == 1 && doc.Title == "" {
				doc.Title = title
			}

			currentSection = &Section{
				Level:    level,
				Title:    title,
				AnchorID: anchorID,
			}
			currentBlocks = make([]Block, 0)

			// Add to TOC
			doc.TOC = append(doc.TOC, TOCEntry{
				Level:    level,
				Title:    title,
				AnchorID: anchorID,
			})
			continue
		}

		// Handle blockquotes
		if strings.HasPrefix(line, "> ") {
			content := strings.TrimPrefix(line, "> ")
			currentBlocks = append(currentBlocks, Block{
				Type:    BlockBlockquote,
				Content: content,
			})
			continue
		}

		// Handle unordered lists
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			item := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			// Check if last block is a list, append to it
			if len(currentBlocks) > 0 && currentBlocks[len(currentBlocks)-1].Type == BlockList {
				currentBlocks[len(currentBlocks)-1].Items = append(
					currentBlocks[len(currentBlocks)-1].Items, item)
			} else {
				currentBlocks = append(currentBlocks, Block{
					Type:  BlockList,
					Items: []string{item},
				})
			}
			continue
		}

		// Handle numbered lists
		numberedListRegex := regexp.MustCompile(`^\d+\.\s+(.+)$`)
		if matches := numberedListRegex.FindStringSubmatch(line); matches != nil {
			item := matches[1]
			if len(currentBlocks) > 0 && currentBlocks[len(currentBlocks)-1].Type == BlockNumberedList {
				currentBlocks[len(currentBlocks)-1].Items = append(
					currentBlocks[len(currentBlocks)-1].Items, item)
			} else {
				currentBlocks = append(currentBlocks, Block{
					Type:  BlockNumberedList,
					Items: []string{item},
				})
			}
			continue
		}

		// Handle images
		imageRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
		if matches := imageRegex.FindStringSubmatch(line); matches != nil {
			currentBlocks = append(currentBlocks, Block{
				Type:    BlockImage,
				Content: matches[2], // image path
			})
			continue
		}

		// Handle regular paragraphs
		if strings.TrimSpace(line) != "" {
			currentBlocks = append(currentBlocks, Block{
				Type:    BlockParagraph,
				Content: line,
			})
		}
	}

	// Save last section
	if currentSection != nil {
		currentSection.Content = currentBlocks
		doc.Sections = append(doc.Sections, *currentSection)
		doc.Anchors[currentSection.AnchorID] = len(doc.Sections) - 1
	}

	return doc, scanner.Err()
}

// ParseMarkdownDir parses all markdown files in a directory
func ParseMarkdownDir(dir string) (*MarkdownDocument, error) {
	doc := &MarkdownDocument{
		Sections: make([]Section, 0),
		TOC:      make([]TOCEntry, 0),
		Anchors:  make(map[string]int),
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}

	// Sort files to ensure consistent order
	// (you might want to use a numeric prefix sorting here)

	for _, file := range files {
		partDoc, err := ParseMarkdownFile(file)
		if err != nil {
			return nil, err
		}

		// Merge sections
		baseIndex := len(doc.Sections)
		for _, section := range partDoc.Sections {
			doc.Sections = append(doc.Sections, section)
			doc.Anchors[section.AnchorID] = baseIndex + len(doc.Sections) - 1
		}

		// Merge TOC
		doc.TOC = append(doc.TOC, partDoc.TOC...)

		// Set title from first document if not set
		if doc.Title == "" {
			doc.Title = partDoc.Title
		}
	}

	return doc, nil
}

// generateAnchorID generates an anchor ID from a heading title
func generateAnchorID(title string) string {
	// Remove markdown formatting
	re := regexp.MustCompile(`[#*_\[\]()]`)
	id := re.ReplaceAllString(title, "")

	// Replace spaces with hyphens
	id = strings.ReplaceAll(strings.TrimSpace(id), " ", "-")

	// Remove consecutive hyphens
	re = regexp.MustCompile(`-+`)
	id = re.ReplaceAllString(id, "-")

	return id
}
