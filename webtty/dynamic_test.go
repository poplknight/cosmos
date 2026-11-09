package webtty

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// 固定一个窗口中心的时间，避免测试跨越窗口边界。
func fixedNow() time.Time { return time.Unix(1700000100, 0) }

func TestCurrentPasswordDeterministicAndRotates(t *testing.T) {
	secret, period := "s3cret-salt", 10*time.Minute
	now := fixedNow()

	a := currentPasswordAt(secret, period, now)
	b := currentPasswordAt(secret, period, now.Add(3*time.Minute)) // 同一窗口内
	if a != b {
		t.Fatalf("同一窗口内密码应一致: %q != %q", a, b)
	}

	c := currentPasswordAt(secret, period, now.Add(period)) // 下一个窗口
	if a == c {
		t.Fatalf("窗口切换后密码应变化: %q", a)
	}

	// 与独立计算对照，验证公式 MD5(窗口字符串+盐)[:6]
	window := now.Truncate(period).Format("060102150405")
	want := fmt.Sprintf("%x", md5.Sum([]byte(window+secret)))[:6]
	if a != want {
		t.Fatalf("公式不符: got %q want %q", a, want)
	}
}

func TestCurrentPasswordRejectsBadPeriod(t *testing.T) {
	if _, err := CurrentPassword("salt", 0); err == nil {
		t.Fatal("period <= 0 应报错")
	}
}

func TestLocalAuthorize(t *testing.T) {
	secret, period := "s3cret-salt", 10*time.Minute
	now := fixedNow()
	fn := func(req Connect) error {
		return authorizeLocalAt(secret, period, now, req)
	}

	// 当前窗口密码通过
	code := currentPasswordAt(secret, period, now)
	if err := fn(Connect{Mode: "local", Password: code}); err != nil {
		t.Fatalf("当前窗口密码应通过: %v", err)
	}

	// 前一个窗口密码也通过（容差）
	prev := currentPasswordAt(secret, period, now.Add(-period))
	if err := fn(Connect{Mode: "local", Password: prev}); err != nil {
		t.Fatalf("前一个窗口密码应通过: %v", err)
	}

	// 后一个窗口密码也通过（容差）
	next := currentPasswordAt(secret, period, now.Add(period))
	if err := fn(Connect{Mode: "local", Password: next}); err != nil {
		t.Fatalf("后一个窗口密码应通过: %v", err)
	}

	// 错误密码拒绝
	if err := fn(Connect{Mode: "local", Password: "000000"}); err == nil {
		t.Fatal("错误密码应被拒绝")
	}

	// 非本机连接放行
	if err := fn(Connect{Mode: "ssh", Host: "x", Password: ""}); err != nil {
		t.Fatalf("非本机连接应放行: %v", err)
	}
}

func TestWithLocalPasswordLocksAfterTenFailures(t *testing.T) {
	srv := New(WithLocalPassword("s3cret-salt", 10*time.Minute))
	ctx := context.Background()
	req := Connect{Mode: "local", Password: "000000"}

	for i := 0; i < 10; i++ {
		if err := srv.authorize(ctx, Meta{}, req); err == nil {
			t.Fatalf("第 %d 次错误应被拒绝", i+1)
		}
	}

	// 第 11 次即使密码正确也被锁定
	code, err := CurrentPassword("s3cret-salt", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.authorize(ctx, Meta{}, Connect{Mode: "local", Password: code}); err == nil {
		t.Fatal("错满 10 次后本窗口应锁定，密码正确也不能登录")
	}
}

func TestLocalLimiterWindowSwitchResets(t *testing.T) {
	l := &localLimiter{limit: 10}
	for i := 0; i < 10; i++ {
		if err := l.judge("w1", false); err == nil {
			t.Fatalf("第 %d 次错误应被拒绝", i+1)
		}
	}
	// w1 已锁定
	if err := l.judge("w1", true); err == nil {
		t.Fatal("w1 锁定期间正确密码也应拒绝")
	}
	// 切到新窗口自动恢复
	if err := l.judge("w2", true); err != nil {
		t.Fatalf("新窗口应自动恢复: %v", err)
	}
	// 成功后计数清零
	for i := 0; i < 3; i++ {
		if err := l.judge("w2", false); err == nil {
			t.Fatal("错误应被拒绝")
		}
	}
	if err := l.judge("w2", true); err != nil {
		t.Fatalf("成功后计数清零，应放行: %v", err)
	}
}

// TestLocalPasswordEndToEnd 验证动态密码完整链路：浏览器把动态密码经加密通道
// 提交 → 服务端解密 → 授权校验 → 打开本机 shell。
func TestLocalPasswordEndToEnd(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	secret, period := "e2e-salt", 10*time.Minute

	srv := New(WithLocalPassword(secret, period))
	page := httptest.NewServer(srv.Mount(""))
	t.Cleanup(page.Close)

	code, err := CurrentPassword(secret, period)
	if err != nil {
		t.Fatal(err)
	}

	ws := dialPath(t, page.URL, "/")
	writeLocal(t, ws, code)
	assertEcho(t, ws)
}

// TestLocalPasswordWrongCodeRejected 验证错误动态密码经完整链路被拒绝。
func TestLocalPasswordWrongCodeRejected(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	secret, period := "e2e-salt", 10*time.Minute

	srv := New(WithLocalPassword(secret, period))
	page := httptest.NewServer(srv.Mount(""))
	t.Cleanup(page.Close)

	ws := dialPath(t, page.URL, "/")
	writeLocal(t, ws, "000000")
	assertError(t, ws, "动态密码错误")
}

func writeLocal(t *testing.T, ws *websocket.Conn, code string) {
	t.Helper()
	pub, err := readHello(t, ws)
	if err != nil {
		t.Fatal(err)
	}
	key, nonce, secret, err := sealLogin(pub, "", code)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(wireConnect{
		Type: "connect", Mode: "local", Cols: 80, Rows: 24,
		Key: key, Nonce: nonce, Secret: secret,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeConnect(t, ws, string(payload))
}
