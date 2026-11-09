package webtty

import (
	"embed"
	"encoding/base64"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

//go:embed static
var uiFS embed.FS

var (
	uiOnce   sync.Once
	uiFiles  http.Handler
	uiAssets = map[string]struct{}{}
	pageHTML []byte
)

func init() {
	_ = fs.WalkDir(uiFS, "static", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(name, "static")
		if !strings.HasPrefix(rel, "/") {
			rel = "/" + rel
		}
		uiAssets[rel] = struct{}{}
		return nil
	})
	pageHTML = assemblePage()
}

// assemblePage puts the stylesheet, script, and icon into the document.
// A router can then expose the whole page on one path; the browser does not
// request style.css or app.js beside that path.
func assemblePage() []byte {
	raw, err := uiFS.ReadFile("static/index.html")
	if err != nil {
		panic(err)
	}
	css, err := uiFS.ReadFile("static/style.css")
	if err != nil {
		panic(err)
	}
	js, err := uiFS.ReadFile("static/app.js")
	if err != nil {
		panic(err)
	}
	icon, err := uiFS.ReadFile("static/favicon.svg")
	if err != nil {
		panic(err)
	}
	page := string(raw)
	page = strings.Replace(page,
		`<link rel="icon" href="favicon.svg" type="image/svg+xml" />`,
		`<link rel="icon" href="data:image/svg+xml;base64,`+base64.StdEncoding.EncodeToString(icon)+`" type="image/svg+xml" />`,
		1)
	page = strings.Replace(page, `<link rel="stylesheet" href="style.css" />`, "<style>\n"+string(css)+"\n</style>", 1)
	page = strings.Replace(page, `<script src="app.js"></script>`, "<script>\n"+strings.ReplaceAll(string(js), "</script>", `<\\/script>`)+"\n</script>", 1)
	return []byte(page)
}

func files() http.Handler {
	uiOnce.Do(func() {
		sub, err := fs.Sub(uiFS, "static")
		if err != nil {
			panic(err)
		}
		uiFiles = http.FileServer(http.FS(sub))
	})
	return uiFiles
}

func serveUI(w http.ResponseWriter, r *http.Request, name string) {
	name = path.Clean("/" + name)
	if name == "/" || name == "/index.html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(pageHTML)
		return
	}
	req := r.Clone(r.Context())
	req.URL.Path = name
	req.URL.RawPath = ""
	w.Header().Set("Cache-Control", "no-cache")
	files().ServeHTTP(w, req)
}

// UI serves the terminal page from the handler root.
// Prefer Mount or MountFastHTTP when the page should share a route with its websocket.
func UI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveUI(w, r, r.URL.Path)
	})
}

var (
	uiFastOnce sync.Once
	uiFast     fasthttp.RequestHandler
)

// UIFastHTTP serves the terminal page on a fasthttp request.
func UIFastHTTP(ctx *fasthttp.RequestCtx) {
	uiFastOnce.Do(func() {
		uiFast = fasthttpadaptor.NewFastHTTPHandler(UI())
	})
	uiFast(ctx)
}

func serveUIFast(ctx *fasthttp.RequestCtx, name string) {
	fasthttpadaptor.NewFastHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveUI(w, r, name)
	}))(ctx)
}
