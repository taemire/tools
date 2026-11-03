package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"report-mcp/artexmd"
	"report-mcp/pentestreport"
)

// The Go renderer can read privateTmp; an otherwise unrestricted resident
// Chrome cannot. macOS seatbelt models the different systemd /tmp views.
func residentWithPrivateTmpDenied(t *testing.T, root, privateTmp string) (string, bool) {
	t.Helper()
	chrome := env("REPORT_TEST_CHROME", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
	if _, e := os.Stat(chrome); e != nil {
		t.Skip("Chrome unavailable")
	}
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	args := []string{"--headless=new", "--no-sandbox", "--remote-debugging-port=" + strconv.Itoa(port), "--remote-debugging-address=127.0.0.1", "--user-data-dir=" + filepath.Join(root, "chrome-profile"), "--no-first-run", "about:blank"}
	denied := runtime.GOOS == "darwin"
	if denied {
		path, _ := json.Marshal(privateTmp)
		profile := "(version 1)(allow default)(deny file-read* (subpath " + string(path) + "))"
		args = append([]string{"-p", profile, chrome}, args...)
		chrome = "/usr/bin/sandbox-exec"
	}
	cmd := exec.Command(chrome, args...)
	log, e := os.Create(filepath.Join(root, "chrome.log"))
	if e != nil {
		t.Fatal(e)
	}
	cmd.Stdout = log
	cmd.Stderr = log
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
		log.Close()
		if dir := os.Getenv("REPORT_PRIVATE_TMP_EVIDENCE"); dir != "" {
			if b, e := os.ReadFile(filepath.Join(root, "chrome.log")); e == nil {
				_ = os.WriteFile(filepath.Join(dir, "sandbox-chrome.log"), b, 0600)
			}
		}
	})
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, e = pentestreport.DevToolsWebSocketURL(ctx, endpoint)
		cancel()
		if e == nil {
			return endpoint, denied
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("resident Chrome did not start")
	return "", false
}

func localCustomerConverter(t *testing.T, root, regular, bold string) string {
	t.Helper()
	src := filepath.Join(root, "converter-source")
	if e := os.CopyFS(src, os.DirFS("../md2pdf")); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command("patch", "--batch", "--forward", "-p1", "-i", mustAbs(t, "scripts/md2pdf-runtime.patch"))
	cmd.Dir = src
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("runtime patch: %v %s", e, b)
	}
	b, e := os.ReadFile("templates/layout_customer.html")
	if e != nil {
		t.Fatal(e)
	}
	// Production uses globally readable /usr/share/fonts. This macOS fixture
	// supplies the same local-file mechanism from the shared test tree.
	s := strings.ReplaceAll(string(b), "/usr/share/fonts/truetype/nanum/NanumGothic.ttf", regular)
	s = strings.ReplaceAll(s, "/usr/share/fonts/truetype/nanum/NanumGothicBold.ttf", bold)
	if e = os.WriteFile(filepath.Join(src, "converter/templates/layout_customer.html"), []byte(s), 0600); e != nil {
		t.Fatal(e)
	}
	binary := filepath.Join(root, "md2pdf")
	cmd = exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = src
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build converter: %v %s", e, b)
	}
	return binary
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	a, e := filepath.Abs(p)
	if e != nil {
		t.Fatal(e)
	}
	return a
}

func TestResidentChromeAcrossPrivateTmp(t *testing.T) {
	root := t.TempDir()
	root, e := filepath.EvalSymlinks(root)
	if e != nil {
		t.Fatal(e)
	}
	privateTmp := filepath.Join(root, "private-tmp")
	shared := filepath.Join(root, "shared")
	os.MkdirAll(privateTmp, 0700)
	os.MkdirAll(shared, 0700)
	endpoint, denied := residentWithPrivateTmpDenied(t, root, privateTmp)
	t.Setenv("REPORT_CHROME_URL", endpoint)
	t.Setenv("REPORT_MD2PDF_DEVTOOLS_PORT", strings.TrimPrefix(endpoint, "http://127.0.0.1:")) // A successful request must reuse this Chrome, not spawn another.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	t.Run("private-file-negative-control", func(t *testing.T) {
		if !denied {
			t.Skip("actual filesystem denial requires macOS seatbelt")
		}
		p := filepath.Join(privateTmp, "hidden.html")
		os.WriteFile(p, []byte("<p>private file negative control</p>"), 0600)
		if _, e := os.ReadFile(p); e != nil {
			t.Fatal("Go cannot read its private file", e)
		}
		_, _, e := pentestreport.PrintPDF(ctx, p, pentestreport.Options{ChromeURL: endpoint})
		if e == nil || (!strings.Contains(e.Error(), "ERR_ACCESS_DENIED") && !strings.Contains(e.Error(), "ERR_FILE_NOT_FOUND")) {
			t.Fatalf("want browser filesystem denial: %v", e)
		}
		t.Logf("Go can read private HTML; resident Chrome refuses: %v", e)
	})
	fonts := filepath.Join(shared, "fonts")
	os.Mkdir(fonts, 0700)
	regular := filepath.Join(fonts, "regular.ttf")
	bold := filepath.Join(fonts, "bold.ttf")
	for target, source := range map[string]string{regular: "pentestreport/assets/fonts/Pretendard-Regular.ttf", bold: "pentestreport/assets/fonts/Pretendard-Bold.ttf"} {
		b, e := os.ReadFile(source)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(target, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	converter := localCustomerConverter(t, shared, regular, bold)
	sessions := filepath.Join(shared, "sessions")
	session := filepath.Join(sessions, "task-1")
	out := filepath.Join(shared, "report-pdf")
	os.MkdirAll(filepath.Join(session, "figures"), 0750)
	os.MkdirAll(out, 0750)
	var picture bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 24, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			im.Set(x, y, color.RGBA{40, 70, 120, 255})
		}
	}
	png.Encode(&picture, im)
	os.WriteFile(filepath.Join(session, "pixel.png"), picture.Bytes(), 0600)
	os.WriteFile(filepath.Join(session, "legacy.md"), []byte("## Shared namespace\n\nLegacy body in the shared job folder.\n\n![Shared image](pixel.png)\n"), 0600)
	manifest := []byte(`{"schema_version":"artex-figure-bundle-1","items":[]}`)
	os.WriteFile(filepath.Join(session, "figures/manifest.json"), manifest, 0600)
	os.WriteFile(filepath.Join(session, "v3.md"), []byte(":::slot meta.cover locked\n## Shared V3\n\nShared resident Chrome document.\n:::\n"), 0600)
	t.Setenv("REPORT_V3_FONT_REGULAR", regular)
	t.Setenv("REPORT_V3_FONT_BOLD", bold)
	a, e := newApp(config{sessions, out, out, converter})
	if e != nil {
		t.Fatal(e)
	}
	defer a.close()
	raw, e := os.ReadFile("pentestreport/testdata/fictional-request.json")
	if e != nil {
		t.Fatal(e)
	}
	var structured renderInput
	if e = json.Unmarshal(raw, &structured); e != nil {
		t.Fatal(e)
	}
	structured.SessionID = "task-1"
	t.Setenv("TMPDIR", privateTmp)
	cases := []struct {
		name  string
		input renderInput
	}{
		{"legacy", renderInput{SessionID: "task-1", MarkdownFile: "legacy.md", Title: "Shared legacy"}},
		{"v3", renderInput{SessionID: "task-1", SourceFormat: "artex-md-3", MarkdownFile: "v3.md", Formats: []string{"pdf", "html"}, RenderInputs: &artexmd.RenderInputs{GeneratedAt: "2026-11-03T00:00:00Z", Title: "Shared V3", HeaderText: "Header", FooterText: "Footer", Language: "en", ManifestSHA256: artexmd.Hash(manifest)}}},
		{"pentest", structured},
	}
	jobs := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, e := a.render(ctx, tc.input)
			if dir := os.Getenv("REPORT_PRIVATE_TMP_EVIDENCE"); dir != "" {
				_ = os.MkdirAll(dir, 0700)
				for _, name := range []string{"render.log", "receipt.json", "report.pdf", "report.html"} {
					if b, err := os.ReadFile(filepath.Join(out, "task-1", r.JobID, name)); err == nil {
						_ = os.WriteFile(filepath.Join(dir, tc.name+"-"+name), b, 0600)
					}
				}
			}
			if e != nil {
				t.Fatal(e)
			}
			if r.Status != "GENERATED_MACHINE_CHECKS_PASS" || r.PDFPath == "" || jobs[r.JobID] {
				t.Fatalf("bad result: status=%s job=%s", r.Status, r.JobID)
			}
			jobs[r.JobID] = true
			b, e := os.ReadFile(r.PDFPath)
			if e != nil || !bytes.HasPrefix(b, []byte("%PDF-")) {
				t.Fatal("no PDF", e)
			}
			if tc.name == "legacy" {
				log, e := os.ReadFile(filepath.Join(out, "task-1", r.JobID, "render.log"))
				if e != nil || !strings.Contains(string(log), "Using running Chrome at "+endpoint) || !strings.Contains(string(log), ".md2pdf-") {
					t.Fatal("converter did not use resident Chrome and job workspace", e)
				}
			}
			if tc.name == "legacy" {
				html, e := os.ReadFile(r.HTMLPath)
				if e != nil || !strings.Contains(string(html), "data:image/png;base64,") {
					t.Fatal("legacy image not embedded", e)
				}
				fontNames, e := exec.Command("pdffonts", r.PDFPath).Output()
				if e != nil || !strings.Contains(string(fontNames), "Pretendard") {
					t.Fatal("shared local-file font not printed", e)
				}
			}
			t.Logf("%s: status=%s pages=%d resident=%s", tc.name, r.Status, r.PageCount, endpoint)
		})
	}
	if e = filepath.WalkDir(out, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".md2pdf-") {
			return fmt.Errorf("workspace retained after render")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(privateTmp)
	expected := 0
	if denied {
		expected = 1
	}
	if e != nil || len(entries) != expected || (denied && entries[0].Name() != "hidden.html") {
		t.Fatal("converter used private TMPDIR", e)
	}
}
