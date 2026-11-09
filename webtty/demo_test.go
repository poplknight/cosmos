package webtty

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDemo 是调用示例。Mount 返回的同一个 HandlerFunc 负责页面和终端：
// 普通 GET 返回页面，带 Upgrade 的 GET 打开 shell。
//
// Gin 注册方式：
//
//	e.GET("/", gin.WrapF(srv.Mount("")))
//
// fasthttp 注册方式：
//
//	handler := srv.MountFastHTTP("")
func TestDemo(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")

	srv := New(WithAuthorize(func(_ context.Context, meta Meta, req Connect) error {
		switch req.Mode {
		case "local":
			if !demoLoopback(meta.RemoteAddr) {
				return errors.New("本机终端只允许本机打开")
			}
			return nil
		case "ssh":
			if req.Username == "" || req.Password == "" {
				return errors.New("缺少登录凭据")
			}
			return nil
		default:
			return errors.New("不支持的连接方式")
		}
	}))

	page := httptest.NewServer(srv.Mount(""))
	t.Cleanup(page.Close)

	res, err := http.Get(page.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), "sealLogin") {
		t.Fatalf("page status %d", res.StatusCode)
	}

	// 浏览器连的是页面地址本身，不是 /ws。
	local := dialPath(t, page.URL, "/")
	writeConnect(t, local, `{"type":"connect","mode":"local","cols":80,"rows":24}`)
	assertEcho(t, local)

	// 钩子返回的错误会原样出现在页面上。未知的 mode 在进入钩子前就会被拒绝。
	blocked := New(WithAuthorize(func(context.Context, Meta, Connect) error {
		return errors.New("本机终端未授权")
	}))
	denied := httptest.NewServer(blocked.Mount(""))
	t.Cleanup(denied.Close)
	rejected := dialPath(t, denied.URL, "/")
	writeConnect(t, rejected, `{"type":"connect","mode":"local","cols":80,"rows":24}`)
	assertError(t, rejected, "本机终端未授权")
}

func demoLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
