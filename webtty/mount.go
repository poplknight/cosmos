package webtty

import (
	"net/http"
	"path"
	"strings"

	"github.com/valyala/fasthttp"
)

// Mount returns a handler for the page and its websocket.
// A browser upgrade on the page path opens the terminal. prefix+"/ws" does the same.
// prefix "/api/v1/xxx" serves the page at /api/v1/xxx/.
func (s *Server) Mount(prefix string) http.HandlerFunc {
	base := cleanMount(prefix)
	socket := s.HTTP()
	return func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if socketPath(p, base, strings.EqualFold(r.Header.Get("Upgrade"), "websocket")) {
			socket(w, r)
			return
		}
		if base != "" && p == base {
			http.Redirect(w, r, base+"/", http.StatusFound)
			return
		}
		if p == joinMount(base, "/") {
			serveUI(w, r, "/")
			return
		}
		name := strings.TrimPrefix(p, base)
		if _, ok := uiAssets[name]; ok && strings.HasPrefix(p, base) {
			serveUI(w, r, name)
			return
		}
		http.NotFound(w, r)
	}
}

// MountFastHTTP returns a handler for the page and its websocket.
// prefix "/api/v1/xxx" serves the page at /api/v1/xxx/ and the socket at /api/v1/xxx/ws.
func (s *Server) MountFastHTTP(prefix string) fasthttp.RequestHandler {
	base := cleanMount(prefix)
	return func(ctx *fasthttp.RequestCtx) {
		p := string(ctx.Path())
		upgrade := strings.EqualFold(string(ctx.Request.Header.Peek("Upgrade")), "websocket")
		if socketPath(p, base, upgrade) {
			s.FastHTTP(ctx)
			return
		}
		if base != "" && p == base {
			ctx.Redirect(base+"/", fasthttp.StatusFound)
			return
		}
		if p == joinMount(base, "/") {
			serveUIFast(ctx, "/")
			return
		}
		name := strings.TrimPrefix(p, base)
		if _, ok := uiAssets[name]; ok && strings.HasPrefix(p, base) {
			serveUIFast(ctx, name)
			return
		}
		ctx.SetStatusCode(fasthttp.StatusNotFound)
	}
}

func socketPath(p, base string, upgrade bool) bool {
	if p == joinMount(base, "/ws") {
		return true
	}
	if !upgrade {
		return false
	}
	return p == joinMount(base, "/") || (base != "" && p == base)
}

func cleanMount(route string) string {
	if route == "" || route == "/" {
		return ""
	}
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	return strings.TrimRight(path.Clean(route), "/")
}

func joinMount(base, suffix string) string {
	if base == "" {
		if suffix == "/" {
			return "/"
		}
		return suffix
	}
	if suffix == "/" {
		return base + "/"
	}
	return base + suffix
}
