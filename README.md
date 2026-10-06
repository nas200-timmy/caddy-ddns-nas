# caddy-ddns-nas

[![CI](https://github.com/nas200-timmy/caddy-ddns-nas/actions/workflows/ci.yml/badge.svg)](https://github.com/nas200-timmy/caddy-ddns-nas/actions/workflows/ci.yml)
[![Release](https://github.com/nas200-timmy/caddy-ddns-nas/actions/workflows/release.yml/badge.svg)](https://github.com/nas200-timmy/caddy-ddns-nas/releases)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

一个静态二进制搞定域名反向代理 + Let's Encrypt 免费证书：TUI 向导配置源站与 DNS 凭证，内置 DDNS 自动更新 A 记录与内嵌 Caddy，支持阿里云 / 腾讯云 DNSPod / Cloudflare。命令名是 `cddns`。

一个静态二进制完成 ddns-go + Caddy 两者组合的效果：在 ECS 或有公网 IPv4 的机器上，
走一遍 Debian 安装器风格的 TUI 向导（源站 → 端口 → DNS 凭证 → 域名），程序自动：

1. 探测本机公网 IP，写入/更新域名的 A 记录（动态 IP 场景由看门狗周期刷新）；
2. 生成 Caddyfile 并以内嵌 Caddy 引擎反向代理域名到源站；
3. 用同一套 DNS 凭证做 ACME DNS-01 验证，自动签发并续期免费证书。

构建产物只有一个可执行文件、单进程：Caddy 以 Go 库形式编译在进程内，
DNS 服务商（阿里云 / 腾讯云 DNSPod / Cloudflare）统一基于 libdns 实现。
二进制约 50MB，静态链接（`CGO_ENABLED=0`），无运行时依赖。

## 快速开始

支持 `linux/amd64`、`linux/arm64`、`linux/armv7`（常见 NAS 与 ARM 服务器）。

```sh
# 方式一：脚本安装指定版本（自动匹配本机架构）
curl -fsSL https://raw.githubusercontent.com/nas200-timmy/caddy-ddns-nas/main/scripts/install.sh \
  | sudo sh -s -- --version v1.0.0

# 方式二：下载发布包手动安装
tar -xzf cddns_v1.0.0_linux_arm64.tar.gz && cd cddns_v1.0.0_linux_arm64
sudo ./install.sh

# 方式三：源码构建（需要 Go ≥ 1.25 工具链）
make build && sudo ./scripts/install.sh
```

安装内容：`/usr/local/bin/cddns`、systemd 单元 `cddns.service`（开机自启）、
配置目录 `/etc/cddns`、数据目录 `/var/lib/cddns`（证书存储）。

```sh
sudo cddns setup   # 配置向导：源站 → 端口 → DNS 凭证 → 域名 → 汇总 → 自动部署
sudo cddns         # 日常管理面板：站点、证书、源站健康状态，Enter 重跑向导
sudo systemctl status cddns   # headless 服务（代理 + DDNS 看门狗）
```

向导部署后服务自动检测到配置变更并热重载（约 5 秒轮询），无需手动重启。
证书由 Caddy 自动续期，无需干预。

## Docker

容器里同样是一个进程，但**必须用 host 网络**：反向代理要监听宿主机 80/443，ACME 也需要真实外部可达端口。

```sh
# 首次配置需要交互终端
docker compose run --rm cddns setup
docker compose up -d
```

或用已发布的多架构镜像：

```sh
docker run -d --name cddns --network host --restart unless-stopped \
  --cap-add NET_BIND_SERVICE \
  -v /etc/cddns:/etc/cddns -v /var/lib/cddns:/var/lib/cddns \
  ghcr.io/nas200-timmy/caddy-ddns-nas:latest
```

配置与证书都在挂载出去的两个目录里，容器重建不丢数据。

从源码构建镜像（墙内需指定 Go 模块代理，否则 `go mod download` 会卡在 proxy.golang.org）：

```sh
docker build --build-arg GOPROXY=https://goproxy.cn,direct -t cddns:latest .
# 或：GOPROXY=https://goproxy.cn,direct docker compose build
```

## 命令与配置

| 命令 | 说明 |
|---|---|
| `cddns setup [--config 路径]` | 配置向导 |
| `cddns run [--config 路径]` | headless 运行（systemd 已托管） |
| `cddns [--config 路径]` | 管理面板 |
| `cddns version` / `cddns --help` | 版本与用法 |

环境变量（容器与非 root 部署用）：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `CDDNS_CONFIG` | `/etc/cddns/config.yaml` | 配置文件路径 |
| `CDDNS_DATA_DIR` | `/var/lib/cddns` | 数据与证书目录 |

## DNS 服务商与最小权限

| 服务商 | 凭证 | 建议权限 |
|---|---|---|
| 阿里云 DNS | RAM 子账户 AccessKey | `AliyunDNSFullAccess` |
| 腾讯云 DNSPod | 子账户 SecretId/SecretKey | `QcloudDNSPodFullAccess` |
| Cloudflare | API Token | Zone.DNS Edit |

凭证仅保存在本机 `/etc/cddns/config.yaml`（权限 0600），
Caddyfile 通过环境变量占位符引用，凭证不会落入 Caddyfile。

## 配置文件

`config.yaml`（向导自动生成，也可手工编辑）：

```yaml
credential:
    provider: alidns          # alidns / dnspod / cloudflare
    secret_id: LTAI****
    secret_key: ****
    # token: ****            # cloudflare 用 token 字段
sites:
    - origin: nas200.top       # 源站 IP 或域名（支持内网 IP / Tailscale / FRP）
      origin_port: 5667        # 源站端口
      origin_scheme: https     # 源站协议：http（默认）/ https
      # origin_tls_insecure: true  # https 源站证书不可信（如自签名）时加这行
      domain: ipv4.nas200.top  # 对外域名
# public_ip: 1.2.3.4          # 可选：手动指定公网 IP（默认自动探测）
# acme_ca: https://...        # 可选：自定义 ACME CA（如 LE staging 先验证）
```

`Caddyfile` 由配置自动生成，勿手工修改。

## 开发

```sh
make build      # 产出 bin/cddns
make test       # 单元 + 内嵌 Caddy 集成测试
make vet        # go vet
make dist       # 交叉编译打包 linux/amd64、arm64、armv7 + checksums.txt
make release    # 扁平发布目录 release/（二进制 + install.sh）
make docker     # buildx 多架构镜像（需要 buildx）
./scripts/smoke_wizard.py   # pty 冒烟测试（需先 make build）
```

- 需要 Go ≥ 1.25（Caddy v2.10 要求）；GOTOOLCHAIN 会自动获取合适工具链。
- 目录：`cmd/`（入口）、`internal/tui`（向导与面板）、`internal/ddns`（libdns 封装与看门狗）、
  `internal/site`（Caddyfile 生成与源站检查）、`internal/caddyembed`（内嵌 Caddy）、
  `internal/config`（配置读写与校验）、`internal/service`（headless 服务与热重载）。
- 提交前会跑 `gofmt`、`go vet`、`go test -race`（见 `.github/workflows/ci.yml`）。
- 打 `v*` tag 会自动交叉编译三个架构、生成 `checksums.txt` 并发布 Release，
  同时把多架构镜像推到 `ghcr.io/nas200-timmy/caddy-ddns-nas`。

## FAQ

- **证书存在哪？** `/var/lib/cddns/caddy/certificates/`（XDG_DATA_HOME 指向处），向导与服务共用。
- **80/443 被占用？** 代理与证书验证需要这两个端口，向导欢迎页会自检并提示。
- **首次签发慢？** DNS 生效有传播延迟，程序会自动等待重试；签发失败会在部署页展示原因。
- **换服务商/换域名？** 重跑 `cddns setup`，配置回填后修改，部署后自动生效。
- **不想装 systemd 单元？** `sudo ./install.sh --dry-run` 先看它要做什么；
  `sudo ./install.sh --uninstall` 卸载（保留配置与证书目录）。
- **要在非默认路径跑？** 用 `--config` 或 `CDDNS_CONFIG` / `CDDNS_DATA_DIR` 覆盖。

## 许可证

本项目采用 [GNU GPL v3.0](LICENSE)。

注意依赖许可的差异：内嵌的 **Caddy 及其部分依赖是 Apache-2.0**，DNS 库多为 MIT。
GPLv3 与 Apache-2.0 兼容，但分发二进制（发布包 / 容器镜像）时仍需一并提供这些组件的许可证文本，
所以打包产物里包含本项目的 `LICENSE`；完整第三方许可证清单尚待补充。
