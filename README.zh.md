<div align="center">

# ddns-watchdog

动态 DNS 更新客户端和中心服务器

[![Language](https://img.shields.io/badge/language-Go-00acd7)](https://go.dev)
[![DeepSource](https://static.deepsource.io/deepsource-badge-light-mini.svg)](https://deepsource.io/gh/y1jiong/ddns-watchdog/?ref=repository-badge)
[![build](https://github.com/y1jiong/ddns-watchdog/actions/workflows/build.yml/badge.svg)](https://github.com/y1jiong/ddns-watchdog/actions/workflows/build.yml)
[![Releases](https://img.shields.io/github/v/release/y1jiong/ddns-watchdog)](https://github.com/y1jiong/ddns-watchdog/releases)
[![Downloads](https://img.shields.io/github/downloads/y1jiong/ddns-watchdog/total)](https://github.com/y1jiong/ddns-watchdog/releases)
[![ClickDownload](https://img.shields.io/badge/%E7%82%B9%E5%87%BB-%E4%B8%8B%E8%BD%BD-brightgreen)](https://github.com/y1jiong/ddns-watchdog/releases)

[安装方式](#安装方式) • [快速上手](#快速上手) • [配置说明](#配置说明) • [支持的服务商](#支持的服务商) • [服务器端](#服务器端) • [从源码编译](#从源码编译)

</div>

## 项目简介

ddns-watchdog 自动检测你的公网 IP，当 IP 发生变化时更新 DNS 解析记录。支持 IPv4 和 IPv6 双栈、多个 DNS 服务商、本地网卡 IP 获取，以及通过中心节点代理管理多台客户端的记录。

## 功能特性

- IPv4 和 IPv6 双栈支持
- 网卡 IP 获取，支持 IPv6 全球单播地址自动回落
- 支持 DNS 服务商：DNSPod、阿里云 DNS、Cloudflare、华为云 DNS
- 中心节点代理模式——一台服务器统一管理多个客户端的 DNS 记录
- 跨平台服务管理：Linux（systemd）、macOS（launchd）、Windows（服务控制管理器）
- Docker 部署，支持通过环境变量完整配置
- 支持周期检查和单次运行两种模式

## 安装方式

### 预编译二进制

从 [Releases](https://github.com/y1jiong/ddns-watchdog/releases) 下载对应平台的压缩包，解压后直接运行。

### Docker Compose

```bash
curl -fsSL https://raw.githubusercontent.com/y1jiong/ddns-watchdog/main/docker-compose.yml -o docker-compose.yml
# 编辑 docker-compose.yml，填入服务商凭证
docker compose up -d
```

### 系统服务

下载或编译好二进制文件后：

```bash
# 初始化配置文件
./ddns-watchdog-client -i 01

# 编辑 conf/client.json 和服务商配置文件，然后安装服务
sudo ./ddns-watchdog-client -I

# 管理服务
sudo systemctl enable --now ddns-watchdog-client
sudo systemctl status ddns-watchdog-client
```

也可以使用 v2 新增的 `service` 子命令：

```bash
sudo ./ddns-watchdog-client service install
sudo ./ddns-watchdog-client service start
sudo ./ddns-watchdog-client service stop
sudo ./ddns-watchdog-client service status
sudo ./ddns-watchdog-client service uninstall
```

## 快速上手

```bash
# 在 ./conf/ 目录初始化所有配置文件
./ddns-watchdog-client -i 01234

# 查看网卡信息（用于网卡 IP 模式）
./ddns-watchdog-client -n

# 立即运行一次
./ddns-watchdog-client

# 强制更新（即使 IP 未变化）
./ddns-watchdog-client -f

# 查看版本并检查更新
./ddns-watchdog-client -V
```

## 配置说明

完整的配置文件格式和环境变量说明请参阅 [docs/config.zh.md](docs/config.zh.md)。

**配置目录：** 默认为 `./conf/`，可通过 `-c <目录>` 指定其他路径。

**优先级：** 命令行参数 > 环境变量 > JSON 配置文件 > 内置默认值

## 支持的服务商

| 服务商 | 配置文件 | 环境变量前缀 |
|---|---|---|
| DNSPod（腾讯云）| `dnspod.json` | `DDNS_DNSPOD_*` |
| 阿里云 DNS | `alidns.json` | `DDNS_ALIDNS_*` |
| Cloudflare | `cloudflare.json` | `DDNS_CLOUDFLARE_*` |
| 华为云 DNS | `huaweicloud.json` | `DDNS_HUAWEI_*` |

各服务商的密钥获取方式请参阅 [docs/provider.zh.md](docs/provider.zh.md)。

## 服务器端

可选的 `ddns-watchdog-server` 二进制提供：
- IP 回显接口（返回调用方的公网 IP 和最新版本号）
- 中心代理接口，多个客户端共享一套 API 凭证

```bash
./ddns-watchdog-server -i 012                                                            # 初始化服务器配置
./ddns-watchdog-server -g -t <token>                                                     # 生成 token
./ddns-watchdog-server -a -t <token> -s dnspod -D example.com -A @ -m "home"            # 添加到白名单
./ddns-watchdog-server                                                                   # 启动服务器
```

## 从源码编译

```bash
git clone https://github.com/y1jiong/ddns-watchdog.git
cd ddns-watchdog
go build ./cmd/ddns-watchdog-client/
go build ./cmd/ddns-watchdog-server/
```

跨平台发布包（需要安装 [goreleaser](https://goreleaser.com)）：

```bash
goreleaser build --snapshot --clean
```
