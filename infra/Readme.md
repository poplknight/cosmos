# infra

cosmos 框架的基础设施层，负责把 **viper 配置装配成运行时句柄**。每个具体模块（logger、后续的数据库连接等）只提供"配置结构 + 构建函数"，由本包统一从配置源加载并注入。

## 快速开始

```go
import (
    "github.com/rs/zerolog/log"

    "github.com/poplknight/cosmos/infra"
)

// 1. 用 path 来源把配置文件加载到 viper 全局实例
v, err := infra.LoadConfig(infra.PathSource("app.yaml"))
if err != nil {
    panic(err)
}

// 2. 从 v 装配 logger（yaml 里有 "logger:" 一节），注入全局 log.Logger
if err := infra.Logger(v); err != nil {
    panic(err)
}

log.Info().Msg("server started on :8787")
```

## 全局 viper

`infra.LoadConfig` 把配置加载进 **viper 库自带的全局实例**（`viper.GetViper()`）并返回它，类似 zerolog 的 `log.Logger`：

- 配置来源由 **`Source`** 抽象：`type Source func(*viper.Viper) error`，一个来源负责把配置应用到 viper
- `LoadConfig` 定位为 **wire provider**，便于后续接入依赖注入
- 返回的 `*viper.Viper` 直接传给各句柄，如 `Logger(v)`；也支持手动构造 viper 传入

## 配置来源

每个来源都有**三层**，按需选择：

| 层 | 作用 | file | nacos | etcd |
|---|---|---|---|---|
| 底层（参数化） | 工程内直接调用、测试最方便 | `PathSource(path)` | `NacosSource(servers, namespace, group, dataID, type)` | `EtcdSource(endpoints, key, type)` |
| 标准 flag 入口 | 请求参数，**内部注册 + Parse** | `FlagPathSource()` | `FlagNacosSource()` | `FlagEtcdSource()` |
| cobra 层 | 从命令 flags 读 | `CmdPathSource(cmd)` | `CmdNacosSource(cmd)` | `CmdEtcdSource(cmd)` |

- `Flag*Source` 是**入口**：调用时注册**该来源自己的 flag**（file 只注册 `-p`；nacos 只注册 `-server/-namespace/-group/-data-id/-type`；etcd 只注册 `-endpoint/-key/-type`）并 `flag.Parse()`，调用方无需预定义或手动解析。三种来源互斥、只用一种，flag 名不需要来源前缀也不会冲突；重复调用会 panic（flag 重定义）——基建配置用错就该直接失败
- cobra 层读同名 flags；`-server`/`-endpoint` 支持逗号分隔多值
- 底层直接传参数；如需把参数组织成 struct，由调用方自己定义（不在这里）
- 统一经 `LoadConfig(source)` 装配；`type` 指定配置格式（yaml/json/toml，空默认 yaml）

配置键为 `logger.level` / `logger.with` / `logger.outputs`；若整个配置嵌套在其它节下，用 `WithLoggerPrefix` 指定外层：

```go
// yaml 里是 app.logger.* 时
if err := infra.Logger(v, infra.WithLoggerPrefix("app")); err != nil {
    panic(err)
}
```

## cobra 层

infra **不提供命令**——命令和启动编排是你程序的职责。它只提供 `Cmd*Source`：在你自己的命令 `RunE` 里读 flags 构造来源：

```go
runCmd := &cobra.Command{
    Use: "run",
    RunE: func(cmd *cobra.Command, _ []string) error {
        source, err := infra.CmdNacosSource(cmd)   // 或 CmdPathSource / CmdEtcdSource
        if err != nil {
            return err
        }
        v, err := infra.LoadConfig(source)
        if err != nil {
            return err
        }
        return infra.Logger(v)                      // 你决定装配哪些句柄
    },
}
runCmd.Flags().String("server", "", "nacos server, host:port (comma separated)")
// ... 其余来源参数 flags（无来源前缀，来源互斥），-server/-endpoint 支持逗号分隔
rootCmd.AddCommand(runCmd)
```

## API

- `infra.LoadConfig(source Source) (*viper.Viper, error)`：用指定来源加载配置到 viper 全局实例并返回（wire provider）
- `infra.PathSource(path string) Source`：本地文件配置来源
- `infra.NacosSource(servers []string, namespace, group, dataID, configType string) Source`：Nacos 配置中心来源
- `infra.EtcdSource(endpoints []string, key, configType string) Source`：etcd 配置来源
- `infra.Logger(v *viper.Viper, opts...) error`：读取 logger 配置、构建 zerolog logger 并注入全局 `log.Logger`
- `infra.WithLoggerPrefix(prefix)`：指定配置外层节（键为 `[prefix.]logger.*`）

## 说明

- logger 的具体配置结构与构建逻辑在 [cosmos/logger](../logger/)，本包只负责"从 viper 找到配置 → 调用构建 → 注入全局"
- 后续新的句柄（数据库连接、缓存等）沿用同一模式追加在这里
