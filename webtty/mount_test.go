package webtty

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/valyala/fasthttp"
)

func TestMount(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	srv := httptest.NewServer(allowAll().Mount("/api/v1/xxx"))
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.Get(srv.URL + "/api/v1/xxx")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/api/v1/xxx/" {
		t.Fatalf("redirect %d %q", res.StatusCode, res.Header.Get("Location"))
	}

	res, err = http.Get(srv.URL + "/api/v1/xxx/")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	if res.StatusCode != http.StatusOK || strings.Contains(page, `src="app.js"`) || !strings.Contains(page, "sealLogin") {
		t.Fatalf("page status %d %s", res.StatusCode, page)
	}

	res, err = http.Get(srv.URL + "/api/v1/xxx/app.js")
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), "location.pathname") {
		t.Fatalf("script status %d", res.StatusCode)
	}

	ws := dialPath(t, srv.URL, "/api/v1/xxx/")
	writeConnect(t, ws, `{"type":"connect","mode":"local","cols":80,"rows":24}`)
	assertEcho(t, ws)
}

func TestMountFastHTTP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpSrv := &fasthttp.Server{Handler: New().MountFastHTTP("/api/v1/xxx")}
	go func() { _ = httpSrv.Serve(ln) }()
	t.Cleanup(func() {
		_ = httpSrv.Shutdown()
		_ = ln.Close()
	})
	base := "http://" + ln.Addr().String()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.Get(base + "/api/v1/xxx")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusFound || !strings.HasSuffix(res.Header.Get("Location"), "/api/v1/xxx/") {
		t.Fatalf("redirect %d %q", res.StatusCode, res.Header.Get("Location"))
	}

	res, err = http.Get(base + "/api/v1/xxx/")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || strings.Contains(string(body), `href="style.css"`) || !strings.Contains(string(body), "--bg:") {
		t.Fatalf("page status %d", res.StatusCode)
	}

	res, err = http.Get(base + "/api/v1/xxx/style.css")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("css status %d", res.StatusCode)
	}

	ws := dialPath(t, base, "/api/v1/xxx/ws")
	writeConnect(t, ws, `{"type":"connect","mode":"local","cols":80,"rows":24}`)
	assertError(t, ws, "未授权")
}

func dialPath(t *testing.T, httpURL, path string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(httpURL, "http") + path
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Close() })
	return ws
}
