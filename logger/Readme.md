# logger

cosmos 框架的日志包。底层由 **zerolog** 驱动，每条日志输出为一行 **JSON**（结构化、高性能）；支持级别过滤、固定字段、多输出（stderr / 文件），文件输出带 **lumberjack 轮转**（按大小切割、保留份数、按天清理、gzip 压缩）。

## 快速开始

```go
import (
    "github.com/rs/zerolog/log"

    "github.com/poplknight/cosmos/infra"
)

// 从 viper 配置加载（yaml 里有 "logger:" 一节），构建的 zerolog logger
// 会被注入到全局 log.Logger，之后整个程序直接用 zerolog 的全局 API：
if err := infra.Logger(v); err != nil {
    panic(err)
}

log.Info().Msg("server started on :8787")
log.Error().Err(err).Msg("connect failed")
```

`Logger` 属于基础设施层（`cosmos/infra`，负责"viper → 句柄"的装配），返回 `error`（logger 已注入全局，无需接收返回值）。配置键为 `logger.level` / `logger.with` / `logger.outputs`；若配置嵌套在其它节下，用 `infra.WithLoggerPrefix` 指定外层：

```go
// yaml 里是 app.logger.* 时
if err := infra.Logger(v, infra.WithLoggerPrefix("app")); err != nil {
    panic(err)
}
```

不想用全局注入、想自己持有 zerolog.Logger 时，直接用 `logger.Config` 构建：

```go
z, err := (logger.Config{Level: "info", Outputs: outputs}).InitLogger() // 返回 zerolog.Logger
```

## 配置

`Config` 三个字段：

| 字段 | 说明 |
|---|---|
| `level` | 最低输出级别：`debug` / `info` / `warn` / `error`（空默认 `info`） |
| `with` | 固定键值字段，附加到每一行日志 |
| `outputs` | 输出列表，可多个（多输出自动聚合） |

yaml 示例：

```yaml
logger:
  level: info
  with:
    app: cosmos
    env: prod
  outputs:
    - kind: stderr
    - kind: file
      config:
        path: /var/log/cosmos.log
        max_size: 100          # 单文件上限 MB，超过即轮转
        max_backups: 7         # 保留 7 个旧文件（0 = 全保留）
        max_age: 30            # 保留 30 天（0 = 按份数/不按天清理）
        compress: true         # 轮转后的文件 gzip 压缩
```

## 输出类型

| kind | 常量 | 配置 | 说明 |
|---|---|---|---|
| `stderr` | `output.KindStderr` | 无 | 写入进程 stderr |
| `file` | `output.KindFile` | `FileConfig` | 写入文件，lumberjack 轮转；未配置 `path` 时回退到 stderr |
| `outputs` | `output.KindOutputs` | `OutputsConfig` | 聚合（嵌套）多个输出；`Config.Outputs` 本身就是一个输出列表 |

`Config.Outputs` 直接是输出数组（`[]OutputConfig`），每行日志写往列表里的每个输出；`kind: outputs` 用于在某个输出元素里再嵌套一层聚合。

配置（yaml）里 `kind` 字段写字符串；代码里构造 `OutputConfig` 时用常量（`Kind` 是带类型的字符串，拼错直接编译不过）：

```go
output.OutputConfig{Kind: output.KindFile, Config: output.FileConfig{Path: path}}
```

每种输出实现 `output.Outputer` 接口（`Config()` + `NewWriteCloser(prefix)`），可自行扩展新后端。

## API

- `logger.Config.InitLogger() (zerolog.Logger, error)`：构建 zerolog logger（含级别过滤、固定字段、输出聚合）
- 从 viper 装配并注入全局 logger 请用 `infra.Logger`（见 [infra](../infra/)），不在此包
- 日志写入全部用 zerolog 原生 API（`log.Info().Msg(...)` / `z.Warn().Str(...).Msg(...)`），低于配置级别的日志在 zerolog 层直接丢弃

输出一行 JSON，例如：

```json
{"level":"info","app":"demo","env":"prod","time":"2026-11-10T12:00:00+08:00","message":"server started on :8787"}
```

`level` 为实际日志级别，`with` 里的固定字段与时间戳随每条日志附带（字段按键名排序注入保证稳定），`message` 为消息内容。低于 `level` 配置的日志在 zerolog 层直接丢弃。
