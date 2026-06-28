<div align="center">

# ddns-watchdog

Dynamic DNS update client and center server

[![Language](https://img.shields.io/badge/language-Go-00acd7)](https://go.dev)
[![DeepSource](https://static.deepsource.io/deepsource-badge-light-mini.svg)](https://deepsource.io/gh/y1jiong/ddns-watchdog/?ref=repository-badge)
[![build](https://github.com/y1jiong/ddns-watchdog/actions/workflows/build.yml/badge.svg)](https://github.com/y1jiong/ddns-watchdog/actions/workflows/build.yml)
[![Releases](https://img.shields.io/github/v/release/y1jiong/ddns-watchdog)](https://github.com/y1jiong/ddns-watchdog/releases)
[![Downloads](https://img.shields.io/github/downloads/y1jiong/ddns-watchdog/total)](https://github.com/y1jiong/ddns-watchdog/releases)
[![ClickDownload](https://img.shields.io/badge/Click-Download-brightgreen)](https://github.com/y1jiong/ddns-watchdog/releases)

[中文](README.zh.md) • [Installation](#installation) • [Quick Start](#quick-start) • [Configuration](#configuration) • [Providers](#supported-providers) • [Server](#server) • [Building](#building-from-source)

</div>

## What is ddns-watchdog?

ddns-watchdog detects your public IP address and updates DNS records automatically when it changes. It supports IPv4 and IPv6, multiple DNS providers, local network card IP detection, and a center proxy mode for managing records from a single server.

## Features

- IPv4 and IPv6 dual-stack support
- Network card IP detection with IPv6 global unicast fallback
- Supported DNS providers: DNSPod, AliDNS, Cloudflare, Huawei Cloud
- Center node proxy — one server manages DNS records for multiple clients
- Cross-platform service management: Linux (systemd), macOS (launchd), Windows (Service Control Manager)
- Docker deployment with full environment variable configuration
- Periodic update or one-shot mode

## Installation

### Prebuilt Binary

Download the archive for your platform from [Releases](https://github.com/y1jiong/ddns-watchdog/releases), extract it, and run the binary directly.

### Docker Compose

```bash
curl -fsSL https://raw.githubusercontent.com/y1jiong/ddns-watchdog/main/docker-compose.yml -o docker-compose.yml
# Edit docker-compose.yml and fill in your provider credentials
docker compose up -d
```

### System Service

After installing or downloading the binary:

```bash
# Initialize config files
./ddns-watchdog-client -i 01

# Edit conf/client.json and your provider's JSON, then install the service
sudo ./ddns-watchdog-client -I

# Manage the service
sudo systemctl enable --now ddns-watchdog-client
sudo systemctl status ddns-watchdog-client
```

Or use the `service` subcommand (v2):

```bash
sudo ./ddns-watchdog-client service install
sudo ./ddns-watchdog-client service start
sudo ./ddns-watchdog-client service stop
sudo ./ddns-watchdog-client service status
sudo ./ddns-watchdog-client service uninstall
```

## Quick Start

```bash
# Initialize all config files in ./conf/
./ddns-watchdog-client -i 01234

# Check your network cards (for network card IP mode)
./ddns-watchdog-client -n

# Run once immediately
./ddns-watchdog-client

# Force-update even if IP hasn't changed
./ddns-watchdog-client -f

# Check version and latest release
./ddns-watchdog-client -V
```

## Configuration

See [docs/config.md](docs/config.md) for a full reference of all JSON config files and their corresponding environment variables.

**Config file location:** `./conf/` by default. Override with `-c <dir>`.

**Priority:** CLI flag > environment variable > JSON file value > built-in default

## Supported Providers

| Provider | Config File | Env Var Prefix |
|---|---|---|
| DNSPod (Tencent) | `dnspod.json` | `DDNS_DNSPOD_*` |
| AliDNS (Alibaba) | `alidns.json` | `DDNS_ALIDNS_*` |
| Cloudflare | `cloudflare.json` | `DDNS_CLOUDFLARE_*` |
| Huawei Cloud | `huaweicloud.json` | `DDNS_HUAWEI_*` |

See [docs/provider.md](docs/provider.md) for per-provider credential setup instructions.

## Server

The optional `ddns-watchdog-server` binary provides:
- An IP echo endpoint (returns the caller's public IP + latest version)
- A center proxy endpoint for multiple clients to share one set of API credentials

```bash
./ddns-watchdog-server -i 012                                                        # initialize server configs
./ddns-watchdog-server -g -t <token>                                                 # generate a token
./ddns-watchdog-server -a -t <token> -s dnspod -D example.com -A @ -m "home"        # add token to whitelist
./ddns-watchdog-server                                                               # start server
```

## Building from Source

```bash
git clone https://github.com/y1jiong/ddns-watchdog.git
cd ddns-watchdog
go build ./cmd/ddns-watchdog-client/
go build ./cmd/ddns-watchdog-server/
```

Cross-platform release archives (requires [goreleaser](https://goreleaser.com)):

```bash
goreleaser build --snapshot --clean
```
