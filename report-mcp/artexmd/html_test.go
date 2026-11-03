package artexmd

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

func testInputs() RenderInputs {
	return RenderInputs{GeneratedAt: "2026-10-17T00:00:00Z", Title: "Synthetic report", HeaderText: "Header & <literal>", FooterText: "Footer </style> & \"literal\"", Language: "ko"}
}
func testFonts() []Font { return []Font{{400, goregular.TTF}, {700, gobold.TTF}} }
func TestHTMLVisibleProjection(t *testing.T) {
	d, e := Parse([]byte(slot("## Synthetic\n\n**bold** *emphasis* `inline` &amp; http://example.invalid/\\<id\\>\\\nnext\n\n3. numbered\n4. next\n\n|label|value|\n|:--|--:|\n|left|right|")))
	if e != nil {
		t.Fatal(e)
	}
	p := d.Projection()
	b, e := HTML(p, testInputs(), nil, testFonts())
	if e != nil {
		t.Fatal(e)
	}
	actual, e := HTMLProjection(b)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := CanonicalJSON(actual)
	want, _ := CanonicalJSON(p)
	if string(a) != string(want) {
		t.Fatalf("got %s want %s", a, want)
	}
	if bytes.Contains(b, []byte(`</style> &`)) {
		t.Fatal("CSS metadata escaped style context")
	}
	for name, mutant := range map[string][]byte{
		"drop value":  bytes.Replace(b, []byte(`>right<`), []byte(`><`), 1),
		"swap labels": bytes.Replace(b, []byte(`>left<`), []byte(`>right<`), 1),
		"list start":  bytes.Replace(b, []byte(`start="3"`), []byte(`start="4"`), 1),
	} {
		got, e := HTMLProjection(mutant)
		data, _ := CanonicalJSON(got)
		if e == nil && string(data) == string(want) {
			t.Fatal("missed mutation", name)
		}
	}
	other, e := HTML(p, testInputs(), nil, testFonts())
	if e != nil || !bytes.Equal(b, other) {
		t.Fatal("non deterministic HTML", e)
	}
}
func TestFigureSnapshotAndReferenceGuards(t *testing.T) {
	tmp := t.TempDir()
	if e := os.MkdirAll(filepath.Join(tmp, "figures"), 0700); e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	raw := append([]byte(nil), b.Bytes()...)
	if e := os.WriteFile(filepath.Join(tmp, "figures/a.png"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	item := FigureItem{ID: "flow-1", FigureID: "exploration_flow", Page: 1, Path: "figures/a.png", PNGSHA256: Hash(raw), Width: 2, Height: 2, SourceModelSHA256: strings.Repeat("a", 64)}
	m := FigureManifest{Schema: "artex-figure-bundle-1", Items: []FigureItem{item}}
	write := func() string {
		b, _ := json.Marshal(m)
		if e := os.WriteFile(filepath.Join(tmp, "figures/manifest.json"), b, 0600); e != nil {
			t.Fatal(e)
		}
		return Hash(b)
	}
	d, e := Parse([]byte(slot("```artex-figure\n{\"schema_version\":\"artex-figure-1\",\"figure_id\":\"exploration_flow\",\"pages\":[{\"page\":1,\"ref\":\"flow-1\"}]}\n```")))
	if e != nil {
		t.Fatal(e)
	}
	root, e := os.OpenRoot(tmp)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	assets, _, e := LoadAssets(root, write(), d)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(tmp, "figures/a.png"), []byte("changed after read"), 0600)
	p := d.ProjectionWithAssets(assets)
	html, e := HTML(p, testInputs(), assets, testFonts())
	if e != nil {
		t.Fatal(e)
	}
	actual, e := HTMLProjection(html)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := CanonicalJSON(actual)
	want, _ := CanonicalJSON(p)
	if string(a) != string(want) {
		t.Fatalf("snapshot/readback mismatch: %s %s", a, want)
	}
	os.WriteFile(filepath.Join(tmp, "figures/a.png"), raw, 0600)
	m.Items[0].Path = "../outside.png"
	if _, _, e = LoadAssets(root, write(), d); e == nil {
		t.Fatal("accepted escaping path")
	}
	m.Items[0] = item
	m.Items = append(m.Items, item)
	if _, _, e = LoadAssets(root, write(), d); e == nil || e.Error() != "FIGURE_REF_DUPLICATE" {
		t.Fatal(e)
	}
}
