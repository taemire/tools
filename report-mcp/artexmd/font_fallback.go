package artexmd

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/image/font/sfnt"
)

// Font data and coverage are generated together from pinned OFL source fonts.
// No system font or network request is used to fill missing characters.
//
//go:embed fallbackfonts/*.woff2 fallbackfonts/*.txt fallbackfonts/manifest.json
var fallbackFiles embed.FS

type FallbackFont struct {
	Family         string
	Weight         int
	Data           []byte
	License        string
	PostScriptName string
}

type fallbackFace struct {
	Weight         int      `json:"weight"`
	File           string   `json:"file"`
	SHA256         string   `json:"sha256"`
	Bytes          int      `json:"bytes"`
	Ranges         [][2]int `json:"ranges"`
	PostScriptName string   `json:"postscript_name"`
	font           FallbackFont
}
type fallbackFamily struct {
	Name     string         `json:"name"`
	Language string         `json:"language"`
	Fonts    []fallbackFace `json:"fonts"`
}

var fallbackOnce sync.Once
var fallbackCatalog []fallbackFamily
var fallbackLoadError error

func fallbackFamilies() ([]fallbackFamily, error) {
	fallbackOnce.Do(func() {
		fail := func() { fallbackLoadError = Error("RENDER_FALLBACK_FONT_INVALID") }
		b, err := fallbackFiles.ReadFile("fallbackfonts/manifest.json")
		if err != nil || Hash(b) != "a4d086ddf28c30ac976f7c6ed6f228d7f5c7f62598e5152c78c361ef27bb2c77" {
			fail()
			return
		}
		var manifest struct {
			Families []fallbackFamily `json:"families"`
		}
		if json.Unmarshal(b, &manifest) != nil || len(manifest.Families) != 2 {
			fail()
			return
		}
		familyNames := map[string]bool{}
		for i := range manifest.Families {
			family := &manifest.Families[i]
			if family.Name != "ReportCJKJP" && family.Name != "ReportCJKSC" || len(family.Fonts) != 2 || familyNames[family.Name] {
				fail()
				return
			}
			familyNames[family.Name] = true
			code := strings.TrimPrefix(family.Name, "ReportCJK")
			license, err := fallbackFiles.ReadFile("fallbackfonts/NotoSans" + code + "-OFL.txt")
			if err != nil {
				fail()
				return
			}
			seen := map[int]bool{}
			for j := range family.Fonts {
				face := &family.Fonts[j]
				if face.Weight != 400 && face.Weight != 700 || seen[face.Weight] || face.File != fmt.Sprintf("NotoSans%s-%d.woff2", code, face.Weight) || path.Base(face.File) != face.File {
					fail()
					return
				}
				seen[face.Weight] = true
				data, err := fallbackFiles.ReadFile("fallbackfonts/" + face.File)
				if err != nil || len(data) != face.Bytes || len(data) < 4 || len(data) > 20<<20 || string(data[:4]) != "wOF2" || Hash(data) != face.SHA256 {
					fail()
					return
				}
				last := -1
				for _, span := range face.Ranges {
					if span[0] <= last || span[0] > span[1] || span[1] > unicode.MaxRune {
						fail()
						return
					}
					last = span[1]
				}
				if face.PostScriptName == "" || strings.ContainsAny(face.PostScriptName, " \r\n\t") {
					fail()
					return
				}
				face.font = FallbackFont{Family: family.Name, Weight: face.Weight, Data: data, License: string(license), PostScriptName: face.PostScriptName}
			}
		}
		fallbackCatalog = manifest.Families
	})
	return fallbackCatalog, fallbackLoadError
}

func (f fallbackFace) covers(r rune) bool {
	i := sort.Search(len(f.Ranges), func(i int) bool { return f.Ranges[i][1] >= int(r) })
	return i < len(f.Ranges) && f.Ranges[i][0] <= int(r)
}

// ResolveFallbackFonts checks both displayed weights before publication. A valid
// PDF container is insufficient evidence that its source characters can render.
func ResolveFallbackFonts(p Projection, in RenderInputs, fonts []Font) ([]FallbackFont, error) {
	text := VisibleText(p) + in.HeaderText + in.FooterText + "0123456789"
	characters := map[rune]bool{}
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return nil, Error("RENDER_GLYPH_UNAVAILABLE")
		}
		if !unicode.IsSpace(r) && !unicode.IsControl(r) && !unicode.In(r, unicode.Cf, unicode.Variation_Selector) {
			characters[r] = true
		}
	}
	missing := map[int]map[rune]bool{}
	for _, face := range fonts {
		font, err := sfnt.Parse(face.Data)
		if err != nil {
			return nil, Error("RENDER_TTF_REQUIRED")
		}
		var buffer sfnt.Buffer
		needed := map[rune]bool{}
		for r := range characters {
			index, err := font.GlyphIndex(&buffer, r)
			if err != nil {
				return nil, Error("RENDER_TTF_REQUIRED")
			}
			if index == 0 {
				needed[r] = true
			}
		}
		missing[face.Weight] = needed
	}
	if missing[400] == nil || missing[700] == nil {
		return nil, Error("RENDER_TTF_REQUIRED")
	}
	if len(missing[400])+len(missing[700]) == 0 {
		return nil, nil
	}
	families, err := fallbackFamilies()
	if err != nil {
		return nil, err
	}
	preferred := "ReportCJKJP"
	if in.Language == "zh" {
		preferred = "ReportCJKSC"
	}
	ordered := append([]fallbackFamily(nil), families...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Name == preferred && ordered[j].Name != preferred })
	var selected []FallbackFont
	for _, family := range ordered {
		used := false
		for _, face := range family.Fonts {
			for r := range missing[face.Weight] {
				if face.covers(r) {
					delete(missing[face.Weight], r)
					used = true
				}
			}
		}
		if used {
			for _, face := range family.Fonts {
				selected = append(selected, face.font)
			}
		}
		if len(missing[400])+len(missing[700]) == 0 {
			return selected, nil
		}
	}
	return selected, Error("RENDER_GLYPH_UNAVAILABLE")
}

// VisibleText excludes protocol identifiers, image contents, and document
// metadata. It follows the values actually emitted by the HTML renderer.
func VisibleText(p Projection) string {
	var out strings.Builder
	var inline func([]Inline)
	inline = func(items []Inline) {
		for _, item := range items {
			out.WriteString(item.Text)
			inline(item.Children)
		}
	}
	var block func(Block)
	block = func(b Block) {
		inline(b.Inlines)
		for _, item := range b.Items {
			for _, part := range item {
				block(part)
			}
		}
		for _, row := range b.Rows {
			for _, cell := range row {
				inline(cell)
			}
		}
		if c := b.Component; c != nil {
			switch c.Schema {
			case "artex-kpi-1":
				for _, item := range c.Items {
					out.WriteString(item.Value)
					out.WriteString(item.Label)
				}
			case "artex-figure-1":
				out.WriteString(c.Eyebrow)
				out.WriteString(c.Title)
			case "artex-detail-1":
				for _, s := range c.Columns {
					out.WriteString(s)
				}
				for _, row := range c.Rows {
					for _, s := range row {
						out.WriteString(s)
					}
				}
			}
		}
	}
	for _, slot := range p.Slots {
		for _, b := range slot.Blocks {
			block(b)
		}
	}
	return out.String()
}

func ExpectedPDFFonts(fonts []Font, fallbacks []FallbackFont) (map[string]bool, error) {
	names := map[string]bool{}
	for _, face := range fonts {
		font, err := sfnt.Parse(face.Data)
		if err != nil {
			return nil, Error("RENDER_TTF_REQUIRED")
		}
		var buffer sfnt.Buffer
		name, err := font.Name(&buffer, sfnt.NameIDPostScript)
		if err != nil || name == "" {
			return nil, Error("RENDER_FONT_NAME_REQUIRED")
		}
		names[name] = true
	}
	for _, face := range fallbacks {
		if face.PostScriptName == "" {
			return nil, Error("RENDER_FONT_NAME_REQUIRED")
		}
		names[face.PostScriptName] = true
	}
	return names, nil
}
