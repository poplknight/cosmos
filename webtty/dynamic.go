package webtty

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CurrentPassword 用盐 secret 和有效期 period 计算当前窗口的 6 位动态密码。
// 部署到客户服务器的 webtty 用同一个盐校验，开发侧现场执行本函数即可拿到运维要用的密码。
func CurrentPassword(secret string, period time.Duration) (string, error) {
	if period <= 0 {
		return "", errors.New("有效期必须大于 0")
	}
	return currentPasswordAt(secret, period, time.Now()), nil
}

func currentPasswordAt(secret string, period time.Duration, now time.Time) string {
	window := now.Truncate(period).Format("060102150405")
	sum := md5.Sum([]byte(window + secret))
	return fmt.Sprintf("%x", sum)[:6]
}

// windowPasswords 返回前一个、当前、后一个三个窗口的动态密码。
func windowPasswords(secret string, period time.Duration, now time.Time) [3]string {
	base := now.Truncate(period)
	return [3]string{
		currentPasswordAt(secret, period, base.Add(-period)),
		currentPasswordAt(secret, period, base),
		currentPasswordAt(secret, period, base.Add(period)),
	}
}

// LocalAuthorize 返回只约束本机连接的授权函数：password 必须等于由盐 secret
// 和有效期 period 算出的动态密码，前一个/当前/后一个三个窗口任一匹配即通过，
// 以容忍生成端与校验端时钟前后漂移一个窗口。非本机连接一律放行。
func LocalAuthorize(secret string, period time.Duration) AuthorizeFunc {
	return func(ctx context.Context, meta Meta, req Connect) error {
		return authorizeLocalAt(secret, period, time.Now(), req)
	}
}

func authorizeLocalAt(secret string, period time.Duration, now time.Time, req Connect) error {
	if req.Mode != "local" {
		return nil
	}
	codes := windowPasswords(secret, period, now)
	if req.Password == codes[0] || req.Password == codes[1] || req.Password == codes[2] {
		return nil
	}
	return errors.New("动态密码错误")
}

// WithLocalPassword 为本机连接启用动态密码鉴权：浏览器提交的动态密码必须等于
// 当前窗口算出的密码（三个窗口容差）。同一窗口内连续错误达到 10 次后，本窗口
// 剩余时间拒绝一切本机连接，下一窗口自动恢复。非本机连接不受影响。
func WithLocalPassword(secret string, period time.Duration) Option {
	limiter := &localLimiter{limit: 10}
	return func(s *Server) {
		if s == nil {
			return
		}
		s.authorize = func(ctx context.Context, meta Meta, req Connect) error {
			if req.Mode != "local" {
				return nil
			}
			window := time.Now().Truncate(period).Format("060102150405")
			codes := windowPasswords(secret, period, time.Now())
			valid := req.Password == codes[0] || req.Password == codes[1] || req.Password == codes[2]
			return limiter.judge(window, valid)
		}
	}
}

// localLimiter 记录同一时间窗口内的错误次数，达到 limit 后锁定本窗口。
type localLimiter struct {
	mu    sync.Mutex
	key   string // 正在计数的窗口
	fails int    // 该窗口内的错误次数
	limit int
}

// judge 判断本次尝试是否放行。valid 表示密码匹配。
// 窗口切换时计数归零；错误满 limit 后本窗口剩余时间一律拒绝，即使密码正确。
func (l *localLimiter) judge(window string, valid bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.key != window {
		l.key = window
		l.fails = 0
	}
	if l.fails >= l.limit {
		return errors.New("动态密码错误次数过多，本窗口已锁定")
	}
	if !valid {
		l.fails++
		return errors.New("动态密码错误")
	}
	l.fails = 0
	return nil
}
