# Configuration Reference

All config files live in the config directory (`./conf/` by default, override with `-c <dir>`).

Environment variables override the corresponding JSON file value. If no JSON file exists, built-in defaults are used. This lets you configure the client entirely via environment variables for Docker deployments.

**Priority:** CLI flag > environment variable > JSON file value > built-in default

---

## client.json

Controls IP detection, DNS provider selection, and update interval.

| JSON field | Type | Default | Env var | Description |
|---|---|---|---|---|
| `api_url.ipv4` | string | `https://yzyweb.cn/ddns-watchdog` | `DDNS_API_URL_IPV4` | API URL to get your IPv4 |
| `api_url.ipv6` | string | `https://yzyweb.cn/ddns-watchdog6` | `DDNS_API_URL_IPV6` | API URL to get your IPv6 |
| `api_url.version` | string | `https://yzyweb.cn/ddns-watchdog` | `DDNS_API_URL_VERSION` | API URL for version check |
| `center.enable` | bool | `false` | `DDNS_CENTER_ENABLE` | Use center proxy mode |
| `center.api_url` | string | `""` | `DDNS_CENTER_URL` | Center server URL |
| `center.token` | string | `""` | `DDNS_CENTER_TOKEN` | Center auth token |
| `enable.ipv4` | bool | `false` | `DDNS_ENABLE_IPV4` | Enable IPv4 updates |
| `enable.ipv6` | bool | `false` | `DDNS_ENABLE_IPV6` | Enable IPv6 updates |
| `network_card.enable` | bool | `false` | `DDNS_NETWORK_CARD_ENABLE` | Get IP from network card |
| `network_card.ipv4` | string | `""` | `DDNS_NETWORK_CARD_IPV4` | Network card key for IPv4 |
| `network_card.ipv6` | string | `""` | `DDNS_NETWORK_CARD_IPV6` | Network card key for IPv6 |
| `services.dnspod` | bool | `false` | `DDNS_SERVICE_DNSPOD` | Enable DNSPod provider |
| `services.alidns` | bool | `false` | `DDNS_SERVICE_ALIDNS` | Enable AliDNS provider |
| `services.cloudflare` | bool | `false` | `DDNS_SERVICE_CLOUDFLARE` | Enable Cloudflare provider |
| `services.huawei_cloud` | bool | `false` | `DDNS_SERVICE_HUAWEI` | Enable Huawei Cloud provider |
| `enable_ipv6_fallback` | bool | `true` | `DDNS_IPV6_FALLBACK` | Fall back to any global unicast IPv6 if preferred NIC has none |
| `check_cycle_minutes` | int | `0` | `DDNS_CHECK_CYCLE` | Update interval in minutes (0 = run once and exit) |

**Example `client.json`:**

```json
{
    "api_url": {
        "ipv4": "https://yzyweb.cn/ddns-watchdog",
        "ipv6": "https://yzyweb.cn/ddns-watchdog6",
        "version": "https://yzyweb.cn/ddns-watchdog"
    },
    "center": {
        "api_url": "",
        "token": "",
        "enable": false
    },
    "enable": {
        "ipv4": true,
        "ipv6": false
    },
    "network_card": {
        "enable": false,
        "ipv4": "",
        "ipv6": ""
    },
    "services": {
        "dnspod": true,
        "alidns": false,
        "cloudflare": false,
        "huawei_cloud": false
    },
    "enable_ipv6_fallback": true,
    "check_cycle_minutes": 5
}
```

---

## dnspod.json

| JSON field | Env var | Description |
|---|---|---|
| `id` | `DDNS_DNSPOD_ID` | DNSPod API ID |
| `token` | `DDNS_DNSPOD_TOKEN` | DNSPod API token |
| `domain` | `DDNS_DNSPOD_DOMAIN` | Root domain, e.g. `example.com` |
| `sub_domain.a` | `DDNS_DNSPOD_SUB_A` | A record subdomain, e.g. `@` or `home` |
| `sub_domain.aaaa` | `DDNS_DNSPOD_SUB_AAAA` | AAAA record subdomain |

---

## alidns.json

| JSON field | Env var | Description |
|---|---|---|
| `access_key_id` | `DDNS_ALIDNS_AK_ID` | Alibaba Cloud Access Key ID |
| `access_key_secret` | `DDNS_ALIDNS_AK_SECRET` | Alibaba Cloud Access Key Secret |
| `domain` | `DDNS_ALIDNS_DOMAIN` | Root domain |
| `sub_domain.a` | `DDNS_ALIDNS_SUB_A` | A record subdomain |
| `sub_domain.aaaa` | `DDNS_ALIDNS_SUB_AAAA` | AAAA record subdomain |

---

## cloudflare.json

| JSON field | Env var | Description |
|---|---|---|
| `zone_id` | `DDNS_CLOUDFLARE_ZONE_ID` | Cloudflare Zone ID |
| `api_token` | `DDNS_CLOUDFLARE_TOKEN` | Cloudflare API token |
| `domain.a` | `DDNS_CLOUDFLARE_DOMAIN_A` | Full domain for A record, e.g. `home.example.com` |
| `domain.aaaa` | `DDNS_CLOUDFLARE_DOMAIN_AAAA` | Full domain for AAAA record |
| `proxied` | `DDNS_CLOUDFLARE_PROXIED` | Enable Cloudflare proxy (orange cloud) |

---

## huaweicloud.json

| JSON field | Env var | Description |
|---|---|---|
| `access_key_id` | `DDNS_HUAWEI_AK_ID` | Huawei Cloud Access Key ID |
| `secret_access_key` | `DDNS_HUAWEI_AK_SECRET` | Huawei Cloud Secret Access Key |
| `zone_name` | `DDNS_HUAWEI_ZONE_NAME` | Zone name with trailing dot, e.g. `example.com.` |
| `domain.a` | `DDNS_HUAWEI_DOMAIN_A` | Full domain for A record, e.g. `home.example.com.` |
| `domain.aaaa` | `DDNS_HUAWEI_DOMAIN_AAAA` | Full domain for AAAA record |

---

## server.json

| JSON field | Env var | Default | Description |
|---|---|---|---|
| `server_addr` | `DDNS_SERVER_ADDR` | `:10032` | Listen address |
| `is_root_server` | `DDNS_SERVER_IS_ROOT` | `false` | Self is the authoritative version source |
| `root_server_url` | `DDNS_SERVER_ROOT_URL` | `https://yzyweb.cn/ddns-watchdog` | Upstream server for version check |
| `center_service` | `DDNS_SERVER_CENTER` | `false` | Enable center proxy endpoint |
| `route.get_ip` | `DDNS_SERVER_ROUTE_GETIP` | `/` | Route for IP echo endpoint |
| `route.center` | `DDNS_SERVER_ROUTE_CENTER` | `/center` | Route for center proxy endpoint |
| `tls.enable` | `DDNS_SERVER_TLS` | `false` | Enable TLS |
| `tls.cert_file` | `DDNS_SERVER_TLS_CERT` | `""` | TLS certificate file path |
| `tls.key_file` | `DDNS_SERVER_TLS_KEY` | `""` | TLS key file path |

---

## services.json

Server-side provider credentials used when center proxy is enabled.

```json
{
    "dnspod": {
        "enable": false,
        "id": "",
        "token": ""
    },
    "alidns": {
        "enable": false,
        "access_key_id": "",
        "access_key_secret": ""
    },
    "cloudflare": {
        "enable": false,
        "zone_id": "",
        "api_token": ""
    },
    "huawei_cloud": {
        "enable": false,
        "access_key_id": "",
        "secret_access_key": ""
    }
}
```

---

## whitelist.json

Authorizes client tokens to update specific DNS records via the center proxy.

```json
{
    "<token>": {
        "enable": true,
        "description": "home-server",
        "service": "dnspod",
        "domain_record": {
            "domain": "example.com",
            "subdomain": {
                "a": "@",
                "aaaa": "@"
            }
        }
    }
}
```

Manage entries with server CLI flags:

```bash
# Add or update
./ddns-watchdog-server -a -t <token> -s dnspod -D example.com -A @ --AAAA @ -m "home"

# Delete
./ddns-watchdog-server -d -t <token>
```
