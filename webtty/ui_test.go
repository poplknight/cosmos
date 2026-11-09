package webtty

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestPageOnSingleRoute(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", New().Mount("/"))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	if res.StatusCode != http.StatusOK || strings.Contains(page, `href="style.css"`) || strings.Contains(page, `src="app.js"`) || !strings.Contains(page, "--bg:") || !strings.Contains(page, "sealLogin") {
		t.Fatalf("status %d page missing inlined assets", res.StatusCode)
	}
}

func TestUI(t *testing.T) {
	srv := httptest.NewServer(UI())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	if res.StatusCode != http.StatusOK || strings.Contains(page, `src="app.js"`) || strings.Contains(page, `href="style.css"`) || !strings.Contains(page, "sealLogin") || strings.Contains(page, "/vendor/") {
		t.Fatalf("status %d page %s", res.StatusCode, page)
	}

	res, err = http.Get(srv.URL + "/style.css")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("css status %d", res.StatusCode)
	}
}

func TestUIFastHTTP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpSrv := &fasthttp.Server{Handler: UIFastHTTP}
	go func() { _ = httpSrv.Serve(ln) }()
	t.Cleanup(func() {
		_ = httpSrv.Shutdown()
		_ = ln.Close()
	})

	res, err := http.Get("http://" + ln.Addr().String() + "/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), "sealLogin") {
		t.Fatalf("status %d body %s", res.StatusCode, body[:min(80, len(body))])
	}
}
