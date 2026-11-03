package main

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"os"
	"strings"

	"golang.org/x/image/font/sfnt"
)

const r3Layout = "artex-content-proof-r3a"

func isR3Layout(s string) bool { return s == r3Layout || s == "artex-layout-r3b-v1" }

type figureCaption struct {
	Kind, Eyebrow, Title, Lede, Footnote, OmitReason string
	Counts                                           map[string]int
}
type figureTextMetric struct {
	Width        float64
	Family       string
	Weight, Size int
}

func r3LoadFaces(in svgBundle, font []byte, paths []string) (map[string]*sfnt.Font, string, error) {
	if len(paths) < 3 || len(in.FontSHA256s) != 4 || !figureHashRE.MatchString(in.TokensSHA256) {
		return nil, "", errors.New("FIGURE_FACE_PROFILE_REQUIRED")
	}
	bytes := map[string][]byte{"sans/400": font}
	for key, path := range map[string]string{"mono/400": paths[0], "sans/500": paths[1], "sans/700": paths[2]} {
		data, e := os.ReadFile(path)
		if e != nil || len(data) > 20<<20 {
			return nil, "", errors.New("FIGURE_WEIGHT_FONT_UNAVAILABLE")
		}
		bytes[key] = data
	}
	faces := map[string]*sfnt.Font{}
	var css strings.Builder
	for _, key := range []string{"sans/400", "sans/500", "sans/700", "mono/400"} {
		data := bytes[key]
		if !figureHashRE.MatchString(in.FontSHA256s[key]) || figureHash(data) != in.FontSHA256s[key] {
			return nil, "", fmt.Errorf("FIGURE_FACE_HASH:%s", key)
		}
		f, e := sfnt.Parse(data)
		if e != nil {
			return nil, "", e
		}
		faces[key] = f
		part := strings.Split(key, "/")
		family := "ARTEXReport"
		if part[0] == "mono" {
			family = "ARTEXMono"
		}
		fmt.Fprintf(&css, `@font-face{font-family:%s;font-weight:%s;src:url(data:font/ttf;base64,%s)}`, family, part[1], base64.StdEncoding.EncodeToString(data))
	}
	return faces, css.String() + `svg{font-synthesis:none;font-kerning:none;font-variant-ligatures:none}`, nil
}
func r3ValidateFaces(f svgFigure, faces map[string]*sfnt.Font) error {
	d := xml.NewDecoder(strings.NewReader(f.SVG))
	var face *sfnt.Font
	var buf sfnt.Buffer
	count := 0
	for {
		token, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "text" {
				family, weight := "sans", "400"
				for _, a := range t.Attr {
					if a.Name.Local == "font-family" && a.Value == "ARTEXMono" {
						family = "mono"
					}
					if a.Name.Local == "font-weight" {
						weight = a.Value
					}
				}
				face = faces[family+"/"+weight]
				if face == nil {
					return errors.New("FIGURE_SYNTHETIC_WEIGHT_REFUSED")
				}
				if count >= len(f.TextMetrics) {
					return errors.New("FIGURE_TEXT_METRICS_REQUIRED")
				}
				metric := f.TextMetrics[count]
				if metric.Family != family || fmt.Sprint(metric.Weight) != weight || math.IsNaN(metric.Width) || math.IsInf(metric.Width, 0) || metric.Width < 0 {
					return errors.New("FIGURE_TEXT_METRIC_PROFILE")
				}
				count++
			}
		case xml.CharData:
			if face != nil {
				for _, r := range string(t) {
					i, e := face.GlyphIndex(&buf, r)
					if e != nil || i == 0 {
						return errors.New("FIGURE_FACE_GLYPH_UNAVAILABLE")
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "text" {
				face = nil
			}
		}
	}
	if count != len(f.TextMetrics) {
		return errors.New("FIGURE_TEXT_METRIC_COUNT")
	}
	return nil
}
func figureCaptionHTML(c *figureCaption) string {
	if c == nil {
		return ""
	}
	return `<p>` + html.EscapeString(c.Eyebrow) + `</p><h2>` + html.EscapeString(c.Title) + `</h2><p>` + html.EscapeString(c.Lede) + `</p>`
}

func figureFootnoteHTML(c *figureCaption) string {
	if c == nil {
		return ""
	}
	return `<p>` + html.EscapeString(c.Footnote) + `</p>`
}

type figureDisplayWarning struct {
	Code, Node, Field, Replacement string
	MessageID                      string `json:"message_id,omitempty"`
}

func r3PreviewHeader(in svgBundle) (string, error) {
	required := []string{"visual.preview.proof_title", "visual.preview.proof_notice", "visual.review.language_warning", "visual.review.translation_draft"}
	for _, id := range required {
		if in.PreviewLabels[id] == "" || strings.Contains(in.PreviewLabels[id], "L10N_KEY_MISSING") {
			return "", fmt.Errorf("L10N_KEY_MISSING:%s", id)
		}
	}
	esc := html.EscapeString
	var out strings.Builder
	out.WriteString(`<h1>` + esc(in.PreviewLabels[required[0]]) + `</h1><p>` + esc(in.PreviewLabels[required[1]]) + `</p>`)
	if in.Language != "ko" {
		out.WriteString(`<aside>` + esc(in.PreviewLabels[required[3]]) + `</aside>`)
	}
	for _, w := range in.ReviewWarnings {
		id := w.MessageID
		if id == "" {
			id = required[2]
		}
		message := in.PreviewLabels[id]
		if message == "" {
			return "", fmt.Errorf("L10N_KEY_MISSING:%s", id)
		}
		out.WriteString(`<aside><code>` + esc(w.Code+" "+w.Node+"/"+w.Field) + `</code><p>` + esc(message) + `</p></aside>`)
	}
	return out.String(), nil
}
