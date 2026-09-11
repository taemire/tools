package converter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUniqueTOCAnchorsAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "same.md"), []byte("# 같은 문서\n\n## 같은 제목\n\n본문\n\n## 같은 제목\n\nEnglish body\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(t.TempDir(), "out.html")
	sections, err := ConvertToHTML(Options{InputDir: dir, OutputFile: output, PDFMode: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 || len(sections[0].SubHeadings) != 2 || len(sections[1].SubHeadings) != 2 {
		t.Fatalf("fixture must yield two document sections with two H2s each: %+v", sections)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, section := range sections {
		all := []string{section.ID}
		for _, sub := range section.SubHeadings {
			all = append(all, sub.ID)
		}
		for _, id := range all {
			if ids[id] {
				t.Errorf("duplicate TOC ID %q", id)
			}
			ids[id] = true
			if n := strings.Count(string(data), `id="`+id+`"`); n != 1 {
				t.Errorf("anchor %q occurs %d times", id, n)
			}
			if !strings.Contains(string(data), `href="#`+id+`"`) {
				t.Errorf("missing TOC link %s", id)
			}
		}
	}
	if len(ids) != 6 {
		t.Fatalf("got %d TOC IDs, want 6", len(ids))
	}
}

func TestMissingPagesFailBeforeHTMLWrite(t *testing.T) {
	for _, pages := range []string{
		`{`, `{"sections":[]}`, `{"sections":[{"id":"doc","page":0}]}`,
		`{"sections":[{"id":"md2pdf-section-1","page":1}]}`,
		`{"sections":[{"id":"md2pdf-section-1","page":1},{"id":"md2pdf-section-1","page":2}]}`,
	} {
		t.Run(pages, func(t *testing.T) {
			dir := t.TempDir()
			input, output, mapping := filepath.Join(dir, "doc.md"), filepath.Join(dir, "out.html"), filepath.Join(dir, "pages.json")
			for path, data := range map[string]string{input: "# Document\n\n## API 설정\n", output: "existing output", mapping: pages} {
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := ConvertToHTML(Options{InputDir: input, OutputFile: output, PagesJSON: mapping, PDFMode: true})
			if err == nil {
				t.Fatal("incomplete pages must fail")
			}
			data, _ := os.ReadFile(output)
			if string(data) != "existing output" {
				t.Fatal("failed conversion replaced output")
			}
		})
	}
}

func TestRejectsAmbiguousSourceAnchors(t *testing.T) {
	for _, content := range []string{
		"# Document\n\n<div id=\"md2pdf-section-1\">collision</div>\n",
		"# Document\n\n<h2 id=\"duplicate\">First</h2>\n<h2 id=\"duplicate\">Second</h2>\n",
	} {
		dir := t.TempDir()
		input := filepath.Join(dir, "doc.md")
		if err := os.WriteFile(input, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ConvertToHTML(Options{InputDir: input, OutputFile: filepath.Join(dir, "out.html")}); err == nil {
			t.Fatal("ambiguous source anchors must fail")
		}
	}
}

func TestExtractMultilineHeadings(t *testing.T) {
	subs := extractSubHeadings("<h2 id=\"long\">긴 제목 English\n<strong>다음 줄 &amp; API</strong></h2>")
	if len(subs) != 1 || !strings.Contains(subs[0].Title, "다음 줄 & API") {
		t.Fatalf("multiline heading lost: %+v", subs)
	}
}

func TestManualTOCOnlyRemovedWhenLinksPresent(t *testing.T) {
	input := "# 문서\n\n## 목차\n- [API 설정](#api)\n\n## API 설정\n본문\n"
	got := stripManualTOC(input)
	if strings.Contains(got, "## 목차") || !strings.Contains(got, "## API 설정") {
		t.Fatal(got)
	}
	plain := "## 목차\n일반 설명입니다.\n"
	if stripManualTOC(plain) != plain {
		t.Fatal("ordinary heading removed")
	}
}
