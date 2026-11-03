package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestV3CapabilitiesOutputSchema(t *testing.T) {
	a, e := newApp(config{t.TempDir(), t.TempDir(), "/public", "/unused"})
	if e != nil {
		t.Fatal(e)
	}
	defer a.close()
	s := a.server()
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	defer httpServer.Close()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_report_result","arguments":{"session_id":"","job_id":"","capabilities_only":true}}}`
	req, e := http.NewRequest(http.MethodPost, httpServer.URL, strings.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	var response struct {
		Error  any `json:"error"`
		Result struct {
			Structured receipt `json:"structuredContent"`
		} `json:"result"`
	}
	if e = json.NewDecoder(res.Body).Decode(&response); e != nil {
		t.Fatal(e)
	}
	if response.Error != nil || response.Result.Structured.Status != "CAPABILITIES" || len(response.Result.Structured.Capabilities) != 4 {
		t.Fatal(response)
	}
}
func TestFigureToolMissingFontsAndSharedBusy(t *testing.T) {
	sessions, out := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(sessions, "task-1"), 0700)
	a, e := newApp(config{sessions, out, "/public", "/unused"})
	if e != nil {
		t.Fatal(e)
	}
	defer a.close()
	bundle := svgBundle{Version: "customer-visuals-1", LayoutVersion: "artex-layout-r3b-v1", ModelSHA256: strings.Repeat("a", 64), Figures: []svgFigure{validFigureForTest()}}
	raw, _ := json.Marshal(bundle)
	os.WriteFile(filepath.Join(sessions, "task-1/bundle.json"), raw, 0600)
	t.Setenv("REPORT_FIGURE_FONT_REGULAR", "")
	if _, e = a.renderReportFigures(context.Background(), reportFiguresInput{SessionID: "task-1", BundleFile: "bundle.json"}); e == nil || e.Error() != "FIGURE_RENDER_UNAVAILABLE" {
		t.Fatal(e)
	}
	a.busy <- struct{}{}
	if _, e = a.renderReportFigures(context.Background(), reportFiguresInput{SessionID: "task-1", BundleFile: "bundle.json"}); e == nil {
		t.Fatal("figure renderer bypassed shared busy lock")
	}
	<-a.busy
	if _, e = a.renderReportFigures(context.Background(), reportFiguresInput{SessionID: "task-1", BundleFile: "../outside.json"}); e == nil || e.Error() != "FIGURE_BUNDLE_MISMATCH" {
		t.Fatal(e)
	}
}
