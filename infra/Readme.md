# infra

cosmos 框架的基础设施层，负责把 **viper 配置装配成运行时句柄**。每个具体模块（logger、后续的数据库连接等）只提供"配置结构 + 构建函数"，由本包统一从配置源加载并注入。

## 快速开始

```go
import (
    "github.com/rs/zerolog/log"

    "github.com/poplknight/cosmos/infra"
)

// 从 viper 配置加载 logger（yaml 里有 "logger:" 一节），
// 构建的 zerolog logger 注入全局 log.Logger：
if err := infra.Logger(v); err != nil {
    panic(err)
}

log.Info().Msg("server started on :8787")
```

配置键为 `logger.level` / `logger.with` / `logger.outputs`；若整个配置嵌套在其它节下，用 `WithLoggerPrefix` 指定外层：

```go
// yaml 里是 app.logger.* 时
if err := infra.Logger(v, infra.WithLoggerPrefix("app")); err != nil {
    panic(err)
}
```

## API

- `infra.Logger(v *viper.Viper, opts...) error`：读取 logger 配置、构建 zerolog logger 并注入全局 `log.Logger`
- `infra.WithLoggerPrefix(prefix)`：指定配置外层节（键为 `[prefix.]logger.*`）

## 说明

- logger 的具体配置结构与构建逻辑在 [cosmos/logger](../logger/)，本包只负责"从 viper 找到配置 → 调用构建 → 注入全局"
- 后续新的句柄（数据库连接、缓存等）沿用同一模式追加在这里
