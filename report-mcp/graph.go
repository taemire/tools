package main

import (
	"fmt"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"unicode/utf8"
)

type graphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
	State string `json:"state,omitempty"`
}
type graphEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}
type reportGraph struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges,omitempty"`
}
type point struct{ x, y int }

func drawLine(img *image.RGBA, a, b point, c color.Color) {
	dx, dy := b.x-a.x, b.y-a.y
	steps := dx
	if steps < 0 {
		steps = -steps
	}
	v := dy
	if v < 0 {
		v = -v
	}
	if v > steps {
		steps = v
	}
	if steps == 0 {
		return
	}
	for i := 0; i <= steps; i++ {
		x, y := a.x+dx*i/steps, a.y+dy*i/steps
		for w := -1; w <= 1; w++ {
			img.Set(x, y+w, c)
		}
	}
}
func graphColumn(kind string) int {
	switch kind {
	case "goal":
		return 0
	case "intent":
		return 1
	default:
		return 2
	}
}
func drawReportGraph(g *reportGraph, output string) error {
	if len(g.Nodes) > 18 || len(g.Edges) > 36 {
		return fmt.Errorf("graph exceeds the customer diagram limit")
	}
	seen := map[string]bool{}
	rows := [3]int{}
	pos := map[string]point{}
	for _, n := range g.Nodes {
		if n.ID == "" || seen[n.ID] || utf8.RuneCountInString(n.Label) > 180 {
			return fmt.Errorf("invalid graph node")
		}
		seen[n.ID] = true
		col := graphColumn(n.Kind)
		pos[n.ID] = point{42 + col*520, 130 + rows[col]*164}
		rows[col]++
	}
	max := 1
	for _, r := range rows {
		if r > max {
			max = r
		}
	}
	if max > 8 {
		return fmt.Errorf("reduce graph to representative relationships")
	}
	img := image.NewRGBA(image.Rect(0, 0, 1600, 185+max*164))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{247, 250, 253, 255}}, image.Point{}, draw.Src)
	fontPath := env("REPORT_FONT", "/usr/share/fonts/truetype/nanum/NanumGothic.ttf")
	b, e := os.ReadFile(fontPath)
	if e != nil {
		return e
	}
	ft, e := opentype.Parse(b)
	if e != nil {
		return e
	}
	face, e := opentype.NewFace(ft, &opentype.FaceOptions{Size: 24, DPI: 96, Hinting: font.HintingFull})
	if e != nil {
		return e
	}
	defer face.Close()
	textAt := func(x, y int, s string, c color.Color) {
		d := font.Drawer{Dst: img, Src: &image.Uniform{c}, Face: face, Dot: fixed.P(x, y)}
		d.DrawString(s)
	}
	ink := color.RGBA{24, 45, 68, 255}
	for i, title := range []string{"점검 목표", "수행한 점검", "관찰 및 결과"} {
		textAt(54+i*520, 67, title, ink)
	}
	edgeColor := color.RGBA{144, 164, 183, 255}
	for _, edge := range g.Edges {
		a, ok := pos[edge.From]
		b, ok2 := pos[edge.To]
		if !ok || !ok2 {
			continue
		}
		a.x += 460
		a.y += 68
		b.y += 68
		if a.x > b.x {
			a.x -= 460
			b.x += 460
		}
		drawLine(img, a, b, edgeColor)
		drawLine(img, b, point{b.x - 10, b.y - 6}, edgeColor)
		drawLine(img, b, point{b.x - 10, b.y + 6}, edgeColor)
	}
	stateLabel := map[string]string{"done": "완료", "met": "확인", "verified": "검증", "confirmed": "확인", "running": "진행 중", "open": "진행 중", "blocked": "보류", "pending": "대기", "created": "대기"}
	for _, n := range g.Nodes {
		p := pos[n.ID]
		c := color.RGBA{222, 233, 246, 255}
		if n.Kind == "finding" {
			c = color.RGBA{250, 225, 221, 255}
		} else if n.Kind == "fact" {
			c = color.RGBA{222, 239, 234, 255}
		}
		draw.Draw(img, image.Rect(p.x, p.y, p.x+460, p.y+138), &image.Uniform{c}, image.Point{}, draw.Src)
		lines := []string{}
		line := ""
		for _, r := range n.Label {
			next := line + string(r)
			if font.MeasureString(face, next).Ceil() > 420 {
				lines = append(lines, line)
				line = string(r)
			} else {
				line = next
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
		if len(lines) > 3 {
			lines = lines[:3]
			lines[2] += "…"
		}
		for i, line := range lines {
			textAt(p.x+16, p.y+34+i*31, line, ink)
		}
		if s := stateLabel[n.State]; s != "" {
			textAt(p.x+16, p.y+127, s, color.RGBA{68, 93, 117, 255})
		}
	}
	textAt(52, img.Bounds().Dy()-24, "관계선은 작업에 저장된 점검·관찰 관계를 나타냅니다.", color.RGBA{81, 101, 121, 255})
	if e = os.MkdirAll(filepath.Dir(output), 0750); e != nil {
		return e
	}
	f, e := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if e != nil {
		return e
	}
	defer f.Close()
	return png.Encode(f, img)
}
