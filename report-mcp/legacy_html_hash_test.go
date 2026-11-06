package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// legacyFakeConverter writes report.pdf and, when html is non-empty,
// report.html beside it, then prints the final TOC marker md2pdf emits.
func legacyFakeConverter(t *testing.T, root, html string) string {
	t.Helper()
	write := ""
	if html != "" {
		write = "printf '%s' '" + html + "' > \"$dir/report.html\"\n"
	}
	script := `#!/bin/sh
set -eu
while [ "$#" -gt 0 ]; do
 if [ "$1" = '-o' ]; then out="$2"; shift 2; else shift; fi
done
dir=$(dirname "$out")
` + write + `printf '%s' 'synthetic, not a PDF' > "$out"
echo '[VALIDATION] Checking final PDF destinations'
`
	converter := filepath.Join(root, "converter.sh")
	if e := os.WriteFile(converter, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	return converter
}

func legacyFixtureApp(t *testing.T, html string) (*app, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX helper")
	}
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	out := filepath.Join(root, "report-pdf")
	os.MkdirAll(filepath.Join(sessions, "conv-7"), 0700)
	os.MkdirAll(out, 0700)
	os.WriteFile(filepath.Join(sessions, "conv-7/input.md"), []byte("## Synthetic\n\nBody."), 0600)
	a, e := newApp(config{sessions, out, out, legacyFakeConverter(t, root, html)})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.close)
	return a, out
}

func storedReceipt(t *testing.T, out, session, job string) map[string]json.RawMessage {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(out, session, job, "receipt.json"))
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]json.RawMessage
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	return m
}

// The chat download gate serves report.html only when the stored receipt's
// v3.html_sha256 equals the file bytes. The Markdown path must record it.
func TestLegacyReceiptRecordsHTMLHash(t *testing.T) {
	const html = "<!doctype html><title>synthetic</title><p>legacy report body</p>"
	a, out := legacyFixtureApp(t, html)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// The fake PDF fails the PDF checks, so the job is HOLD; the receipt is
	// still saved, which is what the HTML hash must be part of.
	r, _ := a.render(ctx, renderInput{SessionID: "conv-7", MarkdownFile: "input.md"})
	if r.JobID == "" || r.Status != "HOLD" {
		t.Fatalf("unexpected result: job=%q status=%q", r.JobID, r.Status)
	}
	disk, e := os.ReadFile(filepath.Join(out, "conv-7", r.JobID, "report.html"))
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(disk)
	want := hex.EncodeToString(sum[:])
	if r.V3 == nil || r.V3.HTMLSHA256 != want {
		t.Fatalf("returned receipt lacks the report.html SHA-256: %+v", r.V3)
	}
	stored := storedReceipt(t, out, "conv-7", r.JobID)
	var v3 map[string]any
	if e = json.Unmarshal(stored["v3"], &v3); e != nil || v3 == nil {
		t.Fatalf("stored receipt has no v3 object: %v", e)
	}
	if v3["html_sha256"] != want {
		t.Fatalf("stored v3.html_sha256 = %v, want %s", v3["html_sha256"], want)
	}
	// Only the HTML hash has a value. source_format stays "" and no artex-md-3
	// provenance is claimed, so consumers that require artex-md-3 refuse it.
	for k, v := range v3 {
		if k != "html_sha256" && v != "" && v != nil {
			t.Fatalf("legacy v3.%s = %v, want empty", k, v)
		}
	}
	for _, k := range []string{"sha256", "html_path", "pdf_path", "source_sha256", "checks", "status"} {
		if _, ok := stored[k]; !ok {
			t.Fatalf("existing receipt field %q missing", k)
		}
	}
}

// validate_report reloads and rewrites the receipt; the HTML hash survives.
func TestLegacyHTMLHashSurvivesReceiptReload(t *testing.T) {
	a, out := legacyFixtureApp(t, "<p>reload</p>")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	r, _ := a.render(ctx, renderInput{SessionID: "conv-7", MarkdownFile: "input.md"})
	before := storedReceipt(t, out, "conv-7", r.JobID)["v3"]
	loaded, e := a.load(resultInput{SessionID: "conv-7", JobID: r.JobID})
	if e != nil {
		t.Fatal(e)
	}
	if e = a.save(loaded); e != nil {
		t.Fatal(e)
	}
	after := storedReceipt(t, out, "conv-7", r.JobID)["v3"]
	if len(before) == 0 || string(before) != string(after) {
		t.Fatalf("v3 changed on reload: %s -> %s", before, after)
	}
}

// Without report.html the job fails as before and no HTML hash is written.
func TestLegacyWithoutHTMLRecordsNoHTMLHash(t *testing.T) {
	a, out := legacyFixtureApp(t, "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	r, e := a.render(ctx, renderInput{SessionID: "conv-7", MarkdownFile: "input.md"})
	if e == nil {
		t.Fatal("render without report.html succeeded")
	}
	if r.V3 != nil {
		t.Fatalf("HTML hash recorded without report.html: %+v", r.V3)
	}
	if b, err := os.ReadFile(filepath.Join(out, "conv-7", r.JobID, "receipt.json")); err == nil {
		var m map[string]json.RawMessage
		if json.Unmarshal(b, &m) == nil {
			if _, ok := m["v3"]; ok {
				t.Fatal("stored receipt has v3 without report.html")
			}
		}
	}
}

// The SDK validates structured output against the schema it infers from
// receipt, through the same AddTool path as render_report. The legacy receipt
// with only v3.html_sha256 set must pass, unchanged, to the client.
func TestLegacyReceiptPassesToolOutputSchema(t *testing.T) {
	a, _ := legacyFixtureApp(t, "<p>schema</p>")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	r, _ := a.render(ctx, renderInput{SessionID: "conv-7", MarkdownFile: "input.md"})
	if r.V3 == nil || r.V3.HTMLSHA256 == "" {
		t.Fatal("legacy receipt has no HTML hash")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "report-pdf", Version: version}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "render_report"}, func(context.Context, *mcp.CallToolRequest, renderInput) (*mcp.CallToolResult, receipt, error) {
		return nil, r, nil
	})
	clientT, serverT := mcp.NewInMemoryTransports()
	ss, e := s.Connect(ctx, serverT, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer ss.Close()
	cs, e := mcp.NewClient(&mcp.Implementation{Name: "schema-check", Version: "1"}, nil).Connect(ctx, clientT, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer cs.Close()
	res, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "render_report", Arguments: map[string]any{"session_id": "conv-7"}})
	if e != nil || res.IsError {
		t.Fatalf("tool output rejected: %v", e)
	}
	b, e := json.Marshal(res.StructuredContent)
	var got receipt
	if e != nil || json.Unmarshal(b, &got) != nil || got.V3 == nil || got.V3.HTMLSHA256 != r.V3.HTMLSHA256 {
		t.Fatalf("structured output lost v3.html_sha256: %s", b)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if e = json.Unmarshal([]byte(text), &got); e != nil || got.V3 == nil || got.V3.HTMLSHA256 != r.V3.HTMLSHA256 {
		t.Fatalf("text result lost v3.html_sha256: %v", e)
	}
}
