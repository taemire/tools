package artexmd

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/png"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type FigureItem struct {
	ID                string `json:"id"`
	FigureID          string `json:"figure_id"`
	Page              int    `json:"page"`
	Path              string `json:"path"`
	PNGSHA256         string `json:"png_sha256"`
	Width             int    `json:"width_px"`
	Height            int    `json:"height_px"`
	SourceModelSHA256 string `json:"source_model_sha256"`
}
type FigureManifest struct {
	Schema string       `json:"schema_version"`
	Items  []FigureItem `json:"items"`
}
type Asset struct {
	Item FigureItem
	PNG  []byte
}

func readFile(root *os.Root, name string, limit int64) ([]byte, error) {
	// 슬래시(/) 논리 경로만 받는다. Windows에서도 같게 판정하도록 path.Clean으로 비교한다.
	if !filepath.IsLocal(name) || name == "." || path.Clean(name) != name || strings.ContainsAny(name, "\\\x00") {
		return nil, Error("FIGURE_PATH_INVALID")
	}
	f, e := root.Open(name)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() || st.Size() > limit {
		return nil, Error("FIGURE_RESOURCE_LIMIT")
	}
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, Error("FIGURE_RESOURCE_LIMIT")
	}
	return b, e
}
func LoadAssets(root *os.Root, manifestHash string, d *Document) (map[string]Asset, FigureManifest, error) {
	var m FigureManifest
	b, e := readFile(root, "figures/manifest.json", 1<<20)
	if e != nil {
		return nil, m, e
	}
	if Hash(b) != manifestHash || !uniqueJSONKeys(b) {
		return nil, m, Error("FIGURE_MANIFEST_INVALID")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if dec.Decode(&m) != nil || dec.Decode(new(any)) != io.EOF || m.Schema != "artex-figure-bundle-1" || len(m.Items) > 50 {
		return nil, m, Error("FIGURE_MANIFEST_INVALID")
	}
	assets := map[string]Asset{}
	var total int
	for _, i := range m.Items {
		if i.ID == "" || i.Page < 1 || i.Width <= 0 || i.Height <= 0 || int64(i.Width)*int64(i.Height) > 40000000 {
			return nil, m, Error("FIGURE_RESOURCE_LIMIT")
		}
		if _, ok := assets[i.ID]; ok {
			return nil, m, Error("FIGURE_REF_DUPLICATE")
		}
		png, e := readFile(root, i.Path, 8<<20)
		if e != nil {
			return nil, m, e
		}
		total += len(png)
		if total > 64<<20 {
			return nil, m, Error("FIGURE_RESOURCE_LIMIT")
		}
		cfg, format, e := image.DecodeConfig(bytes.NewReader(png))
		if e != nil || format != "png" || cfg.Width != i.Width || cfg.Height != i.Height || Hash(png) != i.PNGSHA256 {
			return nil, m, Error("FIGURE_PNG_INVALID")
		}
		assets[i.ID] = Asset{i, png}
	}
	used := map[string]bool{}
	next := 0
	for _, s := range d.Slots {
		for n := s.FirstChild(); n != nil; n = n.NextSibling() {
			// Components are required to be direct slot children.
			for raw, c := range d.Components {
				if raw != n || c.Schema != "artex-figure-1" {
					continue
				}
				for _, p := range c.Pages {
					a, ok := assets[p.Ref]
					if !ok {
						return nil, m, Error("FIGURE_REF_MISSING")
					}
					if used[p.Ref] {
						return nil, m, Error("FIGURE_REF_DUPLICATE")
					}
					if a.Item.FigureID != c.FigureID || a.Item.Page != p.Page || next >= len(m.Items) || m.Items[next].ID != p.Ref {
						return nil, m, Error("FIGURE_REF_MISSING")
					}
					next++
					used[p.Ref] = true
				}
			}
		}
	}
	if len(used) != len(assets) {
		return nil, m, Error("FIGURE_UNREFERENCED")
	}
	return assets, m, nil
}
