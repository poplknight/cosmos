package webtty

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	textMessage   = 1
	binaryMessage = 2
	closeMessage  = 8
	pingMessage   = 9

	closeNormal = 1000
	closePolicy = 1008
)

// Meta is the HTTP request data available to an authorization hook.
type Meta struct {
	RemoteAddr string      // 客户端地址，形如 127.0.0.1:port
	Path       string      // 请求路径
	Header     http.Header // 原始请求头，可用来读取令牌
}

// Connect describes the shell to open. Username and Password are filled only
// after the encrypted login payload has been decrypted.
type Connect struct {
	Mode     string // "local" 打开本机 shell，"ssh" 连接 Host
	Host     string // SSH 地址，省略端口时使用 22
	Username string // 解密后的用户名，本机连接为空
	Password string // 解密后的密码，本机连接为空
	Cols     int    // 终端列数
	Rows     int    // 终端行数
}

// AuthorizeFunc decides whether a connection may open a shell.
// Return a non-nil error to reject it. The error text is sent to the browser.
type AuthorizeFunc func(ctx context.Context, meta Meta, req Connect) error

// Option configures a terminal websocket endpoint.
type Option func(*Server)

// WithAuthorize sets the permission hook. Without it, every connection is rejected.
func WithAuthorize(fn AuthorizeFunc) Option {
	return func(s *Server) {
		if s != nil {
			s.authorize = fn
		}
	}
}

// WithCheckOrigin sets the upgrade origin check.
// Without it, a missing Origin or an Origin whose host matches the request is allowed.
func WithCheckOrigin(fn func(origin, host string) bool) Option {
	return func(s *Server) {
		if s != nil {
			s.checkOrigin = fn
		}
	}
}

// Server serves terminal sessions over a websocket.
// Mount it with HTTP, Mount, FastHTTP, or MountFastHTTP.
type Server struct {
	authorize   AuthorizeFunc
	checkOrigin func(origin, host string) bool
}

// New returns a server that opens a shell only after Authorize allows it.
func New(opts ...Option) *Server {
	s := &Server{}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

type wsConn interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	WriteControl(int, []byte, time.Time) error
	SetReadLimit(int64)
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
	SetPongHandler(func(string) error)
	Close() error
}

func (s *Server) allowOrigin(origin, host string) bool {
	if s != nil && s.checkOrigin != nil {
		return s.checkOrigin(origin, host)
	}
	return defaultOrigin(origin, host)
}

func (s *Server) serve(ctx context.Context, conn wsConn, meta Meta) {
	if conn == nil {
		return
	}
	conn.SetReadLimit(1 << 20)
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		fail(conn, "无法建立加密通道")
		return
	}
	if err = writeHello(conn, priv.PublicKey().Bytes()); err != nil {
		return
	}
	req, err := readConnect(conn, priv)
	if err != nil {
		fail(conn, err.Error())
		return
	}
	if err = s.authorizeRequest(ctx, meta, req); err != nil {
		fail(conn, err.Error())
		return
	}
	if req.Host == "127.0.0.1:22" || req.Host == "localhost:22" {
		req.Mode = "local"
	}

	var sess session
	switch req.Mode {
	case "local":
		sess, err = openLocal(req.Cols, req.Rows)
	case "ssh":
		log.Printf("ssh %s@%s", req.Username, req.Host)
		sess, err = openSSH(req)
	default:
		err = errors.New("未知连接方式")
	}
	if err != nil {
		log.Printf("connect: %v", err)
		fail(conn, err.Error())
		return
	}

	var writeMu sync.Mutex
	writeMsg := func(messageType int, payload []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return conn.WriteMessage(messageType, payload)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		writeMu.Lock()
		_ = conn.Close()
		writeMu.Unlock()
		_ = sess.Close()
	}()

	go pumpOutput(cancel, sess, conn, writeMsg)
	go ping(ctx, cancel, writeMsg)

	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	for {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		switch messageType {
		case textMessage:
			if cols, rows, ok := parseResize(data); ok {
				_ = sess.Resize(cols, rows)
			}
		case binaryMessage:
			if _, err = sess.Write(data); err != nil {
				return
			}
		}
	}
}

func (s *Server) authorizeRequest(ctx context.Context, meta Meta, req Connect) error {
	if s == nil || s.authorize == nil {
		return errors.New("未授权")
	}
	if err := s.authorize(ctx, meta, req); err != nil {
		if err.Error() == "" {
			return errors.New("未授权")
		}
		return err
	}
	return nil
}

func readConnect(conn wsConn, priv *ecdh.PrivateKey) (Connect, error) {
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	messageType, data, err := conn.ReadMessage()
	if err != nil {
		return Connect{}, errors.New("没有收到连接信息")
	}
	if messageType != textMessage {
		return Connect{}, errors.New("请先发送连接信息")
	}
	return parseConnect(priv, data)
}

func fail(conn wsConn, message string) {
	payload, _ := json.Marshal(map[string]string{"type": "error", "message": message})
	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	_ = conn.WriteMessage(textMessage, payload)
	_ = conn.WriteControl(closeMessage, formatCloseMessage(closePolicy, clip(message, 100)), time.Now().Add(time.Second))
	_ = conn.Close()
}

func formatCloseMessage(code int, text string) []byte {
	buf := make([]byte, 2+len(text))
	binary.BigEndian.PutUint16(buf[:2], uint16(code))
	copy(buf[2:], text)
	return buf
}

func clip(message string, max int) string {
	if len(message) <= max {
		return message
	}
	return message[:max]
}

func pumpOutput(cancel context.CancelFunc, reader io.Reader, conn wsConn, writeMsg func(int, []byte) error) {
	buf := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if werr := writeMsg(binaryMessage, buf[:n]); werr != nil {
				cancel()
				return
			}
		}
		if err != nil {
			_ = conn.WriteControl(closeMessage, formatCloseMessage(closeNormal, "ended"), time.Now().Add(time.Second))
			cancel()
			return
		}
	}
}

func ping(ctx context.Context, cancel context.CancelFunc, writeMsg func(int, []byte) error) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := writeMsg(pingMessage, nil); err != nil {
				cancel()
				return
			}
		}
	}
}

func parseResize(data []byte) (int, int, bool) {
	var msg struct {
		Type string `json:"type"`
		Cols int    `json:"cols"`
		Rows int    `json:"rows"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return 0, 0, false
	}
	if msg.Type != "resize" || msg.Cols < 2 || msg.Rows < 2 || msg.Cols > 1000 || msg.Rows > 1000 {
		return 0, 0, false
	}
	return msg.Cols, msg.Rows, true
}
