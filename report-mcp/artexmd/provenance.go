package artexmd

import "embed"

//go:embed html.go docx.go url_text.go
var rendererSources embed.FS

// Hash actual renderer sources rather than a version label or runtime pathname.
func TemplateSHA256() string {
	files := map[string]string{}
	for _, name := range []string{"html.go", "docx.go", "url_text.go"} {
		b, e := rendererSources.ReadFile(name)
		if e != nil {
			return ""
		}
		files[name] = Hash(b)
	}
	b, e := CanonicalJSON(files)
	if e != nil {
		return ""
	}
	return Hash(b)
}
