package renderer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Production 2026-11-02: Chrome advertised ws://[::1]:9223 because another
// Chrome held 127.0.0.1:9223, and the IPv4-only firewall refused the IPv6
// connection. These tests pin the address family to the configured endpoint.

func TestPinWebSocketURLKeepsConfiguredFamily(t *testing.T) {
	for _, adv := range []string{
		"ws://[::1]:9223/devtools/browser/abc",
		"ws://localhost:9223/devtools/browser/abc",
		"ws://127.0.0.1:41000/devtools/browser/abc",
	} {
		got, err := PinWebSocketURL(adv, "127.0.0.1", "9223")
		if err != nil || got != "ws://127.0.0.1:9223/devtools/browser/abc" {
			t.Errorf("%s -> %q, %v", adv, got, err)
		}
	}
	for _, bad := range []string{"http://127.0.0.1:9223/json", "ws://127.0.0.1:9223/devtools/page/1", "::"} {
		if _, err := PinWebSocketURL(bad, "127.0.0.1", "9223"); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestResolveDevToolsPinsIPv6Advertisement(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"Browser":"Chrome/156","webSocketDebuggerUrl":"ws://[::1]:9223/devtools/browser/0b7c"}`)
	}))
	defer srv.Close()
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	ws, err := ResolveDevTools(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if want := "ws://" + net.JoinHostPort(host, port) + "/devtools/browser/0b7c"; ws != want {
		t.Fatalf("got %s, want %s", ws, want)
	}
}

func TestResolveDevToolsRefusesHostNames(t *testing.T) {
	for _, u := range []string{"http://localhost:9223", "http://example.com:9223", "http://10.0.0.5:9223", "ftp://127.0.0.1:9223"} {
		if _, err := ResolveDevTools(context.Background(), u); err == nil || !strings.Contains(err.Error(), "DEVTOOLS_URL_INVALID") {
			t.Errorf("%s: %v", u, err)
		}
	}
}

func TestCheckAdvertisedNamesFamilyMismatch(t *testing.T) {
	if err := CheckAdvertised("ws://127.0.0.1:9223/devtools/browser/x"); err != nil {
		t.Fatal(err)
	}
	for _, ws := range []string{"ws://[::1]:9223/devtools/browser/x", "ws://localhost:9223/devtools/browser/x"} {
		if err := CheckAdvertised(ws); !errors.Is(err, ErrAddressFamily) {
			t.Errorf("%s: %v", ws, err)
		}
	}
}

func TestBusyFixedPortFailsBeforeStartingChrome(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	t.Setenv(EnvChromeURL, "")
	t.Setenv(EnvDevToolsPort, strconv.Itoa(port))
	_, _, _, _, err = browserAllocator(context.Background(), Options{}, nil)
	if !errors.Is(err, ErrPortBusy) {
		t.Fatalf("want %v, got %v", ErrPortBusy, err)
	}
}

func TestUnreachableChromeURLFallsBackToFixedPort(t *testing.T) {
	l, _ := net.Listen("tcp4", "127.0.0.1:0")
	dead := l.Addr().String()
	l.Close()
	free, _ := net.Listen("tcp4", "127.0.0.1:0")
	port := free.Addr().(*net.TCPAddr).Port
	free.Close()
	t.Setenv(EnvChromeURL, "http://"+dead)
	t.Setenv(EnvDevToolsPort, strconv.Itoa(port))
	_, cancel, _, remote, err := browserAllocator(context.Background(), Options{}, nil)
	if err != nil || remote {
		t.Fatal(err, remote)
	}
	cancel()
}

// With a real Chrome: a resident Chrome holds 127.0.0.1:P. md2pdf configured
// with MD2PDF_CHROME_URL reuses it and prints; configured with only the fixed
// port it fails fast with DEVTOOLS_PORT_BUSY instead of connecting to [::1].
func TestRealChromeResidentPort(t *testing.T) {
	chrome := os.Getenv("MD2PDF_TEST_CHROME")
	if chrome == "" {
		chrome = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	}
	if _, err := os.Stat(chrome); err != nil {
		t.Skip("no Chrome for the integration test")
	}
	free, _ := net.Listen("tcp4", "127.0.0.1:0")
	port := free.Addr().(*net.TCPAddr).Port
	free.Close()
	dir := t.TempDir()
	cmd := exec.Command(chrome, "--headless=new", "--remote-debugging-port="+strconv.Itoa(port), "--remote-debugging-address=127.0.0.1",
		"--user-data-dir="+filepath.Join(dir, "profile"), "--no-first-run", "about:blank")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port)
	for i := 0; i < 100; i++ {
		if _, err := ResolveDevTools(context.Background(), endpoint); err == nil {
			break
		}
		exec.Command("sleep", "0.1").Run()
	}
	html := filepath.Join(dir, "in.html")
	os.WriteFile(html, []byte("<!doctype html><meta charset=utf-8><p>resident chrome</p>"), 0o600)

	t.Setenv(EnvChromeURL, "")
	t.Setenv(EnvDevToolsPort, strconv.Itoa(port))
	if err := RenderToPDF(html, filepath.Join(dir, "busy.pdf"), Options{Timeout: 30}); !errors.Is(err, ErrPortBusy) {
		t.Fatalf("fixed busy port: want %v, got %v", ErrPortBusy, err)
	}
	t.Setenv(EnvChromeURL, endpoint)
	out := filepath.Join(dir, "reuse.pdf")
	if err := RenderToPDF(html, out, Options{Timeout: 60}); err != nil {
		t.Fatalf("reuse resident Chrome: %v", err)
	}
	if b, err := os.ReadFile(out); err != nil || !strings.HasPrefix(string(b), "%PDF-") {
		t.Fatal("no PDF from the resident Chrome")
	}
}
