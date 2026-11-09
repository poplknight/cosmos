package webtty

import (
	"context"
	"net/http"

	fws "github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
)

// FastHTTP upgrades a fasthttp request and serves one terminal session.
// Mount it on the path you want; it does not route by itself.
func (s *Server) FastHTTP(ctx *fasthttp.RequestCtx) {
	if s == nil {
		ctx.Error("未授权", fasthttp.StatusUnauthorized)
		return
	}
	up := fws.FastHTTPUpgrader{
		ReadBufferSize:  16 * 1024,
		WriteBufferSize: 16 * 1024,
		CheckOrigin: func(rc *fasthttp.RequestCtx) bool {
			return s.allowOrigin(string(rc.Request.Header.Peek("Origin")), string(rc.Host()))
		},
	}
	meta := Meta{
		RemoteAddr: ctx.RemoteAddr().String(),
		Path:       string(ctx.Path()),
		Header:     headerFromFast(ctx),
	}
	_ = up.Upgrade(ctx, func(conn *fws.Conn) {
		s.serve(context.Background(), conn, meta)
	})
}

func headerFromFast(ctx *fasthttp.RequestCtx) http.Header {
	h := make(http.Header)
	ctx.Request.Header.VisitAll(func(key, value []byte) {
		h.Add(string(key), string(value))
	})
	return h
}
