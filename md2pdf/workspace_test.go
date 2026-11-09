package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPDFWorkspaceLivesBesideOutput(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(root, "private-tmp")
	if e := os.Mkdir(private, 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("TMPDIR", private)
	output := filepath.Join(root, "shared", "new-parent", "report.pdf")
	one, e := createPDFWorkspace(output)
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(one)
	two, e := createPDFWorkspace(output)
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(two)
	if one == two || filepath.Dir(one) != filepath.Dir(output) || filepath.Dir(two) != filepath.Dir(output) {
		t.Fatal("workspace is not job-local and unique")
	}
	for _, p := range []string{one, two} {
		info, e := os.Stat(p)
		if e != nil {
			t.Fatal("workspace stat", e)
		}
		// Windows에는 Unix 권한 비트가 없어 Go가 0777로 보고한다. 권한 비교만 Unix 계열에서 한다.
		// Windows ACL의 안전성은 이 시험으로 검증하지 않는다.
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0700 {
			t.Fatal("workspace permissions", info)
		}
	}
	entries, e := os.ReadDir(private)
	if e != nil || len(entries) != 0 {
		t.Fatal("used private TMPDIR", e)
	}
}

func TestPDFWorkspaceFailureCleansIntermediates(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(root, "private-tmp")
	os.Mkdir(private, 0700)
	t.Setenv("TMPDIR", private)
	output := filepath.Join(root, "shared", "report.pdf")
	os.MkdirAll(filepath.Dir(output), 0700)
	sentinel := []byte("existing published report")
	os.WriteFile(output, sentinel, 0600)
	log, code := runCLI(t, "-i", filepath.Join(root, "missing.md"), "-o", output)
	if code == 0 {
		t.Fatal("missing input succeeded", log)
	}
	b, e := os.ReadFile(output)
	if e != nil || string(b) != string(sentinel) {
		t.Fatal("failed render replaced an existing output")
	}
	for _, dir := range []string{private, filepath.Dir(output)} {
		entries, e := os.ReadDir(dir)
		if e != nil {
			t.Fatal(e)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				t.Fatalf("failed process retained temporary files: %s", entry.Name())
			}
		}
	}
}

func TestPDFWorkspaceChromeFailureCleansGeneratedHTML(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(root, "private-tmp")
	os.Mkdir(private, 0700)
	t.Setenv("TMPDIR", private)
	// A failed remote connection must return through run's deferred cleanup.
	t.Setenv("MD2PDF_CHROME_URL", "http://127.0.0.1:1")
	t.Setenv("MD2PDF_DEVTOOLS_PORT", "")
	input := filepath.Join(root, "source.md")
	os.WriteFile(input, []byte("## Cleanup\n\nGenerated before Chrome fails."), 0600)
	output := filepath.Join(root, "shared", "report.pdf")
	log, code := runCLI(t, "-i", input, "-o", output)
	if code == 0 || !strings.Contains(log, "Pass 1 PDF generation failed") {
		t.Fatal("did not exercise the Chrome failure", code, log)
	}
	for _, dir := range []string{private, filepath.Dir(output)} {
		entries, e := os.ReadDir(dir)
		if e != nil || len(entries) != 0 {
			t.Fatal("Chrome failure retained intermediates", entries, e)
		}
	}
}
