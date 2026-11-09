package webtty

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// HTTP returns the websocket handler for one terminal session.
// Register it with the standard library or wrap it for another router.
func (s *Server) HTTP() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil {
			http.Error(w, "未授权", http.StatusUnauthorized)
			return
		}
		up := websocket.Upgrader{
			ReadBufferSize:  16 * 1024,
			WriteBufferSize: 16 * 1024,
			CheckOrigin: func(r *http.Request) bool {
				return s.allowOrigin(r.Header.Get("Origin"), r.Host)
			},
		}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		meta := Meta{
			RemoteAddr: r.RemoteAddr,
			Path:       r.URL.Path,
			Header:     r.Header.Clone(),
		}
		s.serve(r.Context(), conn, meta)
	}
}
