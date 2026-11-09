# webtty

cosmos 框架的浏览器终端模块。一个 handler 同时返回终端页面并处理 WebSocket，开箱即用，静态资源全部内嵌，无任何外部网络依赖。

## 特性

- **单挂载点**：`srv.Mount("")` 一步挂出页面 + WebSocket，页面连的是页面自己的地址，无需单独注册 `/ws`
- **加密登录**：SSH 用户名密码经 ECDH 密钥交换 + AES-256-GCM 加密后才进入服务端，明文凭据会被拒绝
- **双协议栈**：同时支持 `net/http` 与 fasthttp，各提供路由挂载和裸 handler 两套入口
- **授权与来源检查**：授权钩子默认拒绝一切连接；Origin 检查默认放行同源与本机
- **本机 + SSH**：可打开服务器本机 shell，也可连接任意 SSH 主机
- **零外部资源**：页面、样式、脚本、xterm.js 均内嵌于二进制，可离线部署

## 快速开始

```go
srv := webtty.New(webtty.WithAuthorize(func(ctx context.Context, meta webtty.Meta, req webtty.Connect) error {
	if req.Mode == "local" {
		return errors.New("本机终端未授权")
	}
	return nil
}))
http.Handle("/", srv.Mount(""))
```

不设置 `WithAuthorize` 时所有连接都会被拒绝，包括本机。完整的可运行示例见 [examples/webtty](../examples/webtty/)，测试 `TestDemo` 也可直接运行。

## 接入方式

| 入口 | 路由方式 | 说明 |
|---|---|---|
| `srv.Mount(prefix)` | 自带路由 | 返回 `http.HandlerFunc`；`prefix+"/"` 出页面，`prefix+"/ws"` 处理 WebSocket，浏览器升级请求走页面路径 |
| `srv.MountFastHTTP(prefix)` | 自带路由 | fasthttp 等价物 |
| `srv.HTTP()` | 不自带路由 | 单个 WebSocket 会话的 `http.HandlerFunc`，挂到你自己的路径上 |
| `srv.FastHTTP(ctx)` | 不自带路由 | fasthttp 单会话 handler |
| `webtty.UI()` / `webtty.UIFastHTTP()` | 只出页面 | 页面单独挂载，适合已有 WebSocket 路由的框架 |

挂载前缀规则（`Mount` / `MountFastHTTP`）：

- `prefix = ""`：页面在 `/`，资源在 `/xterm.css` 等
- `prefix = "/term"`：页面在 `/term/`，WebSocket 在 `/term/ws`
- `prefix = "/api/v1/xxx"`：页面在 `/api/v1/xxx/`，WebSocket 在 `/api/v1/xxx/ws`
- 访问 `prefix` 本身会 302 到 `prefix/`

集成第三方路由：

```go
// Gin：handler 是 http.HandlerFunc，直接使用
ginEngine.Any("/term/*rest", gin.WrapH(srv.Mount("/term")))
```

## 配置

```go
type Option func(*Server)

webtty.WithAuthorize(fn webtty.AuthorizeFunc)  // 授权钩子，决定是否允许打开 shell
webtty.WithCheckOrigin(fn func(origin, host string) bool) // 自定义 WebSocket Origin 检查
```

- `webtty.Meta`：授权钩子拿到的请求信息（客户端地址、路径、请求头），可读取令牌
- `webtty.Connect`：解密后的连接请求（连接方式、主机、用户名密码、终端行列数）

## 本机动态密码鉴权

`mode = "local"` 是本机 shell，部署到客户服务时可用**动态密码**保护：密码随时间变化，只有持有盐（secret）的人能现场算出当前密码，客户无法自行进入。

```go
salt := "部署方自己定义的一串字符"

// 服务器端：本机连接必须提交正确的动态密码，SSH 连接不受影响
srv := webtty.New(webtty.WithLocalPassword(salt, 10*time.Minute))

// 开发现场给运维算当前密码（6 位）
code, err := webtty.CurrentPassword(salt, 10*time.Minute)
```

原理：`密码 = MD5( 当前窗口串 + 盐 )[:6]`，窗口串 = `time.Now().Truncate(有效期).Format("060102150405")`。校验取**前一个、当前、后一个**三个窗口的密码比对，容忍生成端与校验端时钟漂移一个窗口；同一窗口内连续错误满 10 次后，本窗口剩余时间拒绝一切本机连接，下一窗口自动恢复。

相关 API：

- `webtty.WithLocalPassword(secret string, period time.Duration) webtty.Option`：启用本机动态密码鉴权（含错误锁定）
- `webtty.LocalAuthorize(secret string, period time.Duration) webtty.AuthorizeFunc`：不带锁定的纯校验函数，可与自定义授权逻辑组合
- `webtty.CurrentPassword(secret string, period time.Duration) (string, error)`：算当前动态密码，开发侧现场执行

部署与使用流程：

1. 部署方自定一串盐，配置到客户服务器的 webtty（见 [examples/webtty](../examples/webtty/)）
2. 运维需要进入时找开发；开发现场执行 `webtty.CurrentPassword`（示例程序传 `-code`）算出当前密码
3. 运维在浏览器点"本机"，输入动态密码进入

## 连接方式

- **`mode = "local"`**：打开服务器本机 shell（优先 `$SHELL`，缺省 `/bin/zsh`），通过 PTY 分配终端
- **`mode = "ssh"`**：连接 `Host`（缺省端口 22），支持密码认证与键盘交互认证
- 特殊规则：`Host` 为 `127.0.0.1:22` 或 `localhost:22` 时自动按本机连接处理

## 安全说明

- **授权**：必须设置 `WithAuthorize`，否则拒绝一切连接。授权钩子的错误文本会原样返回浏览器，请勿在其中泄漏敏感信息
- **凭据传输**：握手后服务端先下发 ECDH 公钥，客户端将用户名密码用共享密钥 AES-256-GCM 加密后回传；WebSocket 明文帧里的 `username`/`password` 字段会被直接拒绝
- **Origin 检查**：默认允许「无 Origin」「Origin 主机与请求主机一致」「Origin 为本机（localhost / 127.0.0.1 / ::1）」；跨站页面无法发起 WebSocket 升级
- **SSH 主机密钥**：固定接受任意主机密钥，不做 known_hosts 校验。这是刻意的简化——本模块定位为部署到客户内网、由客户临时使用的运维 shell，连接目标都是客户自己的机器，省去密钥分发。请勿将其直接暴露到公网

## 浏览器端协议（摘要）

服务端握手流程：`hello`（下发公钥）→ 客户端加密 `connect` → 服务端打开 shell → 双向二进制流。

- `text` 消息：客户端 `{"type":"resize","cols":N,"rows":N}` 调整终端尺寸
- `binary` 消息：shell 的输入输出字节流
- 心跳：服务端每 20 秒发送 ping 帧

## 依赖

Go 依赖：`gorilla/websocket`、`fasthttp/websocket`、`creack/pty`、`golang.org/x/crypto/ssh`。

前端资源（内嵌于 `static/`，均已本地化，无 CDN 请求）：

- xterm.js 5.3.0（MIT，版权归 xterm.js 作者）
- xterm-addon-fit 0.8.0（MIT）
