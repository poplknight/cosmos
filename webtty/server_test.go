package webtty

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/valyala/fasthttp"
)

func TestParseConnectRejectsPlaintext(t *testing.T) {
	_, err := parseConnect(nil, []byte(`{"type":"connect","mode":"ssh","host":"10.0.0.8","username":"root","password":"pw"}`))
	if err == nil || !strings.Contains(err.Error(), "明文") {
		t.Fatalf("got %v", err)
	}
}

func TestSSHAddress(t *testing.T) {
	cases := []struct{ in, out string }{
		{"10.0.0.8", "10.0.0.8:22"},
		{"10.0.0.8:2222", "10.0.0.8:2222"},
		{"example.com", "example.com:22"},
		{"::1", "[::1]:22"},
		{"[::1]", "[::1]:22"},
	}
	for _, tc := range cases {
		got, err := sshAddress(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		if got != tc.out {
			t.Fatalf("%s: got %s want %s", tc.in, got, tc.out)
		}
	}
}

func TestNilHookRejectsLocal(t *testing.T) {
	srv := httpServer(t, New())
	ws := dial(t, srv.URL, nil)
	writeConnect(t, ws, `{"type":"connect","mode":"local","cols":80,"rows":24}`)
	assertError(t, ws, "未授权")
}

func TestHookCanRejectLocal(t *testing.T) {
	s := New(WithAuthorize(func(context.Context, Meta, Connect) error {
		return errors.New("不允许连接本机")
	}))
	srv := httpServer(t, s)
	ws := dial(t, srv.URL, nil)
	writeConnect(t, ws, `{"type":"connect","mode":"local","cols":80,"rows":24}`)
	assertError(t, ws, "不允许连接本机")
}

func TestHookSeesRequest(t *testing.T) {
	var got Meta
	var mode, user, pass string
	s := New(WithAuthorize(func(_ context.Context, meta Meta, req Connect) error {
		got = meta
		mode = req.Mode
		user = req.Username
		pass = req.Password
		return errors.New("停在钩子")
	}))
	srv := httpServer(t, s)
	header := http.Header{"Authorization": []string{"Bearer secret"}}
	ws := dial(t, srv.URL, header)
	writeSSH(t, ws, "10.0.0.8", "root", "pw")
	assertError(t, ws, "停在钩子")
	if mode != "ssh" || user != "root" || pass != "pw" || got.Header.Get("Authorization") != "Bearer secret" {
		t.Fatalf("hook saw mode %q user %q pass %q auth %q", mode, user, pass, got.Header.Get("Authorization"))
	}
}

func TestHTTPLocalEcho(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	srv := httpServer(t, allowAll())
	ws := dial(t, srv.URL, nil)
	writeConnect(t, ws, `{"type":"connect","mode":"local","cols":80,"rows":24}`)
	assertEcho(t, ws)
}

func TestFastHTTPLocalEcho(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	url := fastServer(t, allowAll())
	ws := dial(t, url, nil)
	writeConnect(t, ws, `{"type":"connect","mode":"local","cols":80,"rows":24}`)
	assertEcho(t, ws)
}

func TestFastHTTPRejectsWithoutHook(t *testing.T) {
	url := fastServer(t, New())
	ws := dial(t, url, nil)
	writeConnect(t, ws, `{"type":"connect","mode":"local","cols":40,"rows":12}`)
	assertError(t, ws, "未授权")
}

func TestSSHRefused(t *testing.T) {
	srv := httpServer(t, allowAll())
	ws := dial(t, srv.URL, nil)
	writeSSH(t, ws, "127.0.0.1:1", "root", "secret")
	assertError(t, ws, "连接被拒绝")
}

func TestPlaintextPasswordRejected(t *testing.T) {
	srv := httpServer(t, allowAll())
	ws := dial(t, srv.URL, nil)
	writeConnect(t, ws, `{"type":"connect","mode":"ssh","host":"127.0.0.1:1","username":"root","password":"secret","cols":80,"rows":24}`)
	assertError(t, ws, "凭据不能明文传输")
}

func allowAll() *Server {
	return New(WithAuthorize(func(context.Context, Meta, Connect) error { return nil }))
}

func httpServer(t *testing.T, s *Server) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(s.HTTP())
	t.Cleanup(srv.Close)
	return srv
}

func fastServer(t *testing.T, s *Server) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpSrv := &fasthttp.Server{Handler: s.FastHTTP}
	go func() { _ = httpSrv.Serve(ln) }()
	t.Cleanup(func() {
		_ = httpSrv.Shutdown()
		_ = ln.Close()
	})
	return "http://" + ln.Addr().String()
}

func dial(t *testing.T, httpURL string, header http.Header) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(httpURL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Close() })
	return ws
}

func writeConnect(t *testing.T, ws *websocket.Conn, payload string) {
	t.Helper()
	if err := ws.WriteMessage(websocket.TextMessage, []byte(payload)); err != nil {
		t.Fatal(err)
	}
}

func assertError(t *testing.T, ws *websocket.Conn, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_ = ws.SetReadDeadline(deadline)
		_, data, err := ws.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"type":"hello"`) {
			continue
		}
		if !strings.Contains(string(data), want) {
			t.Fatalf("reply %s, want %s", data, want)
		}
		return
	}
	t.Fatal("no error message")
}

func writeSSH(t *testing.T, ws *websocket.Conn, host, username, password string) {
	t.Helper()
	pub, err := readHello(t, ws)
	if err != nil {
		t.Fatal(err)
	}
	key, nonce, secret, err := sealLogin(pub, username, password)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(wireConnect{
		Type: "connect", Mode: "ssh", Host: host, Cols: 80, Rows: 24,
		Key: key, Nonce: nonce, Secret: secret,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeConnect(t, ws, string(payload))
}

func readHello(t *testing.T, ws *websocket.Conn) ([]byte, error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_ = ws.SetReadDeadline(deadline)
		_, data, err := ws.ReadMessage()
		if err != nil {
			return nil, err
		}
		var msg struct {
			Type      string `json:"type"`
			PublicKey string `json:"publicKey"`
		}
		if json.Unmarshal(data, &msg) != nil || msg.Type != "hello" {
			continue
		}
		return decodeKey(msg.PublicKey)
	}
	return nil, errors.New("no hello")
}

func assertEcho(t *testing.T, ws *websocket.Conn) {
	t.Helper()
	if err := ws.WriteMessage(websocket.BinaryMessage, []byte("printf 'marker-%s\\n' ok\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var got strings.Builder
	for time.Now().Before(deadline) {
		_ = ws.SetReadDeadline(deadline)
		kind, data, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v; so far %q", err, got.String())
		}
		if kind != websocket.BinaryMessage {
			continue
		}
		got.Write(data)
		if strings.Contains(got.String(), "marker-ok") {
			return
		}
	}
	t.Fatalf("marker not found in %q", got.String())
}

func TestFastHTTPHeaderCopied(t *testing.T) {
	var auth string
	s := New(WithAuthorize(func(_ context.Context, meta Meta, _ Connect) error {
		auth = meta.Header.Get("X-Token")
		return errors.New("停")
	}))
	url := fastServer(t, s)
	header := http.Header{"X-Token": []string{"abc"}}
	ws := dial(t, url, header)
	writeConnect(t, ws, `{"type":"connect","mode":"local","cols":80,"rows":24}`)
	assertError(t, ws, "停")
	if auth != "abc" {
		t.Fatalf("token %q", auth)
	}
}
