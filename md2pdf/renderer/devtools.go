package renderer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// Environment settings for the browser connection. They are read when the
// caller leaves the matching Options field empty.
//
//	MD2PDF_CHROME_URL      DevTools HTTP endpoint of a running Chrome to reuse,
//	                       for example http://127.0.0.1:9223 (numeric loopback only).
//	MD2PDF_DEVTOOLS_PORT   fixed DevTools port for the Chrome md2pdf starts itself.
//	                       The port is bound on 127.0.0.1 only.
const (
	EnvChromeURL    = "MD2PDF_CHROME_URL"
	EnvDevToolsPort = "MD2PDF_DEVTOOLS_PORT"
)

// ErrAddressFamily marks a DevTools address on a different loopback family
// than the one md2pdf may use (for example Chrome advertising [::1] while
// only 127.0.0.1 is allowed).
var ErrAddressFamily = errors.New("DEVTOOLS_ADDRESS_FAMILY_MISMATCH")

// ErrPortBusy marks a fixed DevTools port that is already in use on 127.0.0.1.
var ErrPortBusy = errors.New("DEVTOOLS_PORT_BUSY")

// loopbackEndpoint parses a DevTools HTTP endpoint and requires a numeric
// loopback host, so the address family is fixed by configuration.
func loopbackEndpoint(endpoint string) (host, port string, err error) {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "ws") || u.Host == "" {
		return "", "", fmt.Errorf("DEVTOOLS_URL_INVALID: %q must be http://<loopback-ip>:<port>", endpoint)
	}
	host, port, err = net.SplitHostPort(u.Host)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		return "", "", fmt.Errorf("DEVTOOLS_URL_INVALID: %q must use a numeric loopback address, not a host name", endpoint)
	}
	return host, port, nil
}

// PinWebSocketURL returns the advertised browser WebSocket URL with its host
// and port replaced by the configured endpoint's numeric host and port.
// Chrome builds the advertised URL from its own listening address, which can
// be a different address family than the one the client may reach.
func PinWebSocketURL(advertised, host, port string) (string, error) {
	w, err := url.Parse(advertised)
	if err != nil || (w.Scheme != "ws" && w.Scheme != "wss") || !strings.HasPrefix(w.Path, "/devtools/browser/") {
		return "", fmt.Errorf("DEVTOOLS_URL_INVALID: unexpected WebSocket URL %q", advertised)
	}
	w.Host = net.JoinHostPort(host, port)
	return w.String(), nil
}

// ResolveDevTools reads /json/version from a running Chrome and returns the
// browser WebSocket URL pinned to the endpoint's address.
func ResolveDevTools(ctx context.Context, endpoint string) (string, error) {
	host, port, err := loopbackEndpoint(endpoint)
	if err != nil {
		return "", err
	}
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(c, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/json/version", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("DEVTOOLS_UNREACHABLE: %s: %w", net.JoinHostPort(host, port), err)
	}
	defer resp.Body.Close()
	var v struct {
		WS string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&v); err != nil || v.WS == "" {
		return "", fmt.Errorf("DEVTOOLS_UNREACHABLE: %s/json/version has no webSocketDebuggerUrl", net.JoinHostPort(host, port))
	}
	return PinWebSocketURL(v.WS, host, port)
}

// checkPortFree fails when 127.0.0.1:port is already bound. Chrome would
// otherwise fall back to [::1]:port and advertise an IPv6 WebSocket address.
func checkPortFree(port int) error {
	l, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("%w: 127.0.0.1:%d is in use; set %s to reuse that Chrome or stop it", ErrPortBusy, port, EnvChromeURL)
	}
	return l.Close()
}

var listeningRE = regexp.MustCompile(`DevTools listening on (ws://\S+)`)

// outputWatch collects Chrome's startup output to report the advertised
// DevTools address when the connection fails.
type outputWatch struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *outputWatch) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() < 64<<10 {
		w.buf.Write(p)
	}
	return len(p), nil
}

func (w *outputWatch) advertised() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if m := listeningRE.FindSubmatch(w.buf.Bytes()); m != nil {
		return string(m[1])
	}
	return ""
}

// CheckAdvertised returns ErrAddressFamily when the WebSocket URL Chrome
// advertised is not on the IPv4 loopback address md2pdf requested.
func CheckAdvertised(ws string) error {
	if ws == "" {
		return nil
	}
	u, err := url.Parse(ws)
	if err != nil {
		return fmt.Errorf("DEVTOOLS_URL_INVALID: %q", ws)
	}
	if ip := net.ParseIP(u.Hostname()); ip == nil || ip.To4() == nil || !ip.IsLoopback() {
		return fmt.Errorf("%w: Chrome advertised %s, but md2pdf connects only to 127.0.0.1 (the IPv4 port was probably taken, so Chrome listened on another address)", ErrAddressFamily, ws)
	}
	return nil
}

// isolatedContext returns a tab context for one render. With a reused
// (remote) Chrome the tab lives in a new browser context created with
// disposeOnDetach, so no cookies, storage or cache are shared with other
// users of that browser and everything is discarded when the render ends.
// The target is created in a new window: Chrome 156 refuses a tab in a new
// browser context that has no window ("no browser is open").
func isolatedContext(allocCtx context.Context, remote bool, opts ...chromedp.ContextOption) (context.Context, context.CancelFunc, error) {
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx, opts...)
	if !remote {
		return browserCtx, cancelBrowser, nil
	}
	if err := chromedp.Run(browserCtx); err != nil {
		cancelBrowser()
		return nil, nil, err
	}
	c := chromedp.FromContext(browserCtx)
	exec := cdp.WithExecutor(browserCtx, c.Browser)
	bc, err := target.CreateBrowserContext().WithDisposeOnDetach(true).Do(exec)
	if err != nil {
		cancelBrowser()
		return nil, nil, err
	}
	id, err := target.CreateTarget("about:blank").WithBrowserContextID(bc).WithNewWindow(true).Do(exec)
	if err != nil {
		_ = target.DisposeBrowserContext(bc).Do(exec)
		cancelBrowser()
		return nil, nil, err
	}
	tab, cancelTab := chromedp.NewContext(browserCtx, chromedp.WithTargetID(id))
	return tab, func() {
		cancelTab()
		_ = target.DisposeBrowserContext(bc).Do(exec)
		cancelBrowser()
	}, nil
}

// browserAllocator chooses how to reach Chrome:
//  1. a running Chrome at MD2PDF_CHROME_URL (WebSocket URL pinned to that address), when reachable;
//  2. otherwise a new Chrome on MD2PDF_DEVTOOLS_PORT bound to 127.0.0.1 after checking the port is free;
//  3. otherwise a new Chrome on a random port (chromedp default).
//
// The returned diagnose function turns a connection error into one that names
// the advertised address and the address-family mismatch; remote reports a
// reused Chrome.
//
// Trust: a reused Chrome is reached over an unauthenticated loopback DevTools
// port. Use MD2PDF_CHROME_URL only on a host where that port belongs to a
// Chrome dedicated to the report service (single-tenant host, UID firewall);
// do not use it on shared multi-user hosts.
func browserAllocator(ctx context.Context, opts Options, flags []chromedp.ExecAllocatorOption) (context.Context, context.CancelFunc, func(error) error, bool, error) {
	chromeURL := opts.ChromeURL
	if chromeURL == "" {
		chromeURL = os.Getenv(EnvChromeURL)
	}
	port := opts.DevToolsPort
	if port == 0 {
		if v := os.Getenv(EnvDevToolsPort); v != "" {
			p, err := strconv.Atoi(v)
			if err != nil || p < 1 || p > 65535 {
				return nil, nil, nil, false, fmt.Errorf("DEVTOOLS_PORT_INVALID: %s=%q", EnvDevToolsPort, v)
			}
			port = p
		}
	}
	pass := func(err error) error { return err }
	if chromeURL != "" {
		ws, err := ResolveDevTools(ctx, chromeURL)
		if err == nil {
			fmt.Printf("[INFO] Using running Chrome at %s\n", chromeURL)
			a, c := chromedp.NewRemoteAllocator(ctx, ws)
			return a, c, pass, true, nil
		}
		if port == 0 || strings.HasPrefix(err.Error(), "DEVTOOLS_URL_INVALID") {
			return nil, nil, nil, false, err
		}
		fmt.Printf("[INFO] %v; starting a private Chrome on 127.0.0.1:%d\n", err, port)
	}
	if port == 0 {
		a, c := chromedp.NewExecAllocator(ctx, flags...)
		return a, c, pass, false, nil
	}
	if err := checkPortFree(port); err != nil {
		return nil, nil, nil, false, err
	}
	watch := &outputWatch{}
	flags = append(flags,
		chromedp.Flag("remote-debugging-port", strconv.Itoa(port)),
		chromedp.Flag("remote-debugging-address", "127.0.0.1"),
		chromedp.CombinedOutput(watch))
	a, c := chromedp.NewExecAllocator(ctx, flags...)
	diagnose := func(err error) error {
		if err == nil {
			return nil
		}
		if e := CheckAdvertised(watch.advertised()); e != nil {
			return fmt.Errorf("%w (%v)", e, err)
		}
		return err
	}
	return a, c, diagnose, false, nil
}
