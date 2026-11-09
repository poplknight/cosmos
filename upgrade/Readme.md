# upgrade

cosmos 框架的版本化升级包。注册"版本 → 升级动作"清单，`Up` 从当前版本逐级升到最新，适合部署在客户服务上、需要原地升级的程序。

## 快速开始

```go
// Store 由用户实现：当前版本存哪（文件、数据库、远端…）
type Store interface {
    GetVersion(ctx context.Context) (string, error)
    SetVersion(ctx context.Context, version string) error
}

err := upgrade.New(store).
    Register("v0.0.1", func(ctx context.Context) error { /* 升到 v0.0.1 */ }).
    Register("v0.0.2", func(ctx context.Context) error { /* 升到 v0.0.2 */ }).
    Up(ctx)
```

## 版本与映射

- 对外版本形式为 `vMAJOR.MINOR.PATCH`（如 `v0.0.1`、`v1.2.3`）
- 包内部用 `int64` 做排序与推进，仿 IP 点分十进制编码，**每段 16 位**：

```
v0.0.0   → 0
v0.0.5   → 5
v1.0.0   → 1 << 32
v1.2.3   → 1<<32 | 2<<16 | 3
```

- 每段范围 0–65535，超界报错；编码保证版本的字典序与数值序一致
- `SetVersion` 写回时自动还原成 `vMAJOR.MINOR.PATCH` 字符串

## `Up` 行为

- 从 `Store.GetVersion` 读当前版本，**空字符串视为 `v0.0.0`**（首次部署）
- 已注册版本升序执行，只执行 `> 当前版本` 的；**允许跳跃**（注册 `v0.0.1` 与 `v0.0.3`、当前 `v0.0.0` → 依次执行 `v0.0.1` 再 `v0.0.3`）
- 每个升级函数**成功后才 `SetVersion` 推进**当前版本；任一失败即停，版本停留在失败前，下次 `Up` 从断点续跑
- 升级函数的**原子性与幂等由函数自己保证**（改配置、写文件等动作不可回滚）
- 内部加锁，重复调用 `Up` 安全

## `Register`

- 返回自身以支持链式调用；版本格式错误、重复注册、函数为 nil 都会 **panic**——清单是静态写死的，应在初始化阶段尽早暴露
- 注册建议在初始化阶段完成，`Up` 阶段运行

## cobra 子命令

`upgrade.Command(u *Upgrade)` 返回一个 cobra 子命令，挂到自己的根命令上即可手动执行升级：

```go
m := upgrade.New(store).Register("v0.0.1", fn1).Register("v0.0.2", fn2)
rootCmd.AddCommand(upgrade.Command(m))
```

| 子命令 | 作用 |
|---|---|
| `yourcli upgrade` | 执行从当前版本到最新的升级，完成后打印 `已升级到 vX.Y.Z` |
| `yourcli upgrade current` | 打印当前版本（未初始化显示 `v0.0.0`） |

`upgrade` 命令在升级失败时返回错误，退出码非 0。

## 错误语义

- 升级函数失败：`升级到 vX.Y.Z 失败: <原因>`，`Up` 返回错误，版本不推进
- 写回版本失败：`写入版本 vX.Y.Z 失败: <原因>`（升级动作已执行，需自行确认幂等）
