package artexmd

import (
	"html"
	"regexp"
	"strings"
)

// The legacy preview keeps its own byte-stable helper. This v3 equivalent emits
// XHTML void tags so independent visible-value extraction preserves text order.
var urlText = regexp.MustCompile(`(?m)https?://[^\s]+|(?:^|[ \t])/[A-Za-z0-9_<>{}%][^\s]*`)

func urlHTML(s string) string {
	var b strings.Builder
	pos := 0
	for _, span := range urlText.FindAllStringIndex(s, -1) {
		b.WriteString(html.EscapeString(s[pos:span[0]]))
		token := s[span[0]:span[1]]
		start := 0
		for i, r := range token {
			if strings.ContainsRune("/?&.", r) {
				b.WriteString(`<span class="url-piece">` + html.EscapeString(token[start:i+1]) + `</span><wbr/>`)
				start = i + 1
			}
		}
		if start < len(token) {
			b.WriteString(`<span class="url-piece">` + html.EscapeString(token[start:]) + `</span>`)
		}
		pos = span[1]
	}
	b.WriteString(html.EscapeString(s[pos:]))
	return b.String()
}
