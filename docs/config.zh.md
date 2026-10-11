# 配置参考

所有配置文件存放在配置目录中（默认为 `./conf/`，可用 `-c <目录>` 指定）。

环境变量会覆盖 JSON 配置文件中对应的值。

`client.json` 或 `server.json` 不存在不算错误，此时使用下表中的默认值，因此可以完全
通过环境变量完成配置（Docker 镜像就是这么用的）。服务商配置文件没有默认值，因此每个
必需的值要么写在对应服务商的 JSON 文件里，要么写在环境变量里。

**优先级：** 命令行参数 > 环境变量 > JSON 配置文件值 > 内置默认值

---

## client.json

控制 IP 检测、DNS 服务商选择和更新间隔。

| JSON 字段               | 类型   | 默认值                             | 环境变量                   | 说明                                       |
|-------------------------|--------|------------------------------------|----------------------------|--------------------------------------------|
| `api_url.ipv4`          | string | `https://yzyweb.cn/ddns-watchdog`  | `DDNS_API_URL_IPV4`        | 获取 IPv4 的 API 地址                      |
| `api_url.ipv6`          | string | `https://yzyweb.cn/ddns-watchdog6` | `DDNS_API_URL_IPV6`        | 获取 IPv6 的 API 地址                      |
| `api_url.version`       | string | `https://yzyweb.cn/ddns-watchdog`  | `DDNS_API_URL_VERSION`     | 版本检查 API 地址                          |
| `center.enable`         | bool   | `false`                            | `DDNS_CENTER_ENABLE`       | 启用中心代理模式                           |
| `center.api_url`        | string | `""`                               | `DDNS_CENTER_URL`          | 中心服务器地址                             |
| `center.token`          | string | `""`                               | `DDNS_CENTER_TOKEN`        | 中心认证 token                             |
| `enable.ipv4`           | bool   | `false`                            | `DDNS_ENABLE_IPV4`         | 启用 IPv4 更新                             |
| `enable.ipv6`           | bool   | `false`                            | `DDNS_ENABLE_IPV6`         | 启用 IPv6 更新                             |
| `network_card.enable`   | bool   | `false`                            | `DDNS_NETWORK_CARD_ENABLE` | 从网卡获取 IP                              |
| `network_card.ipv4`     | string | `""`                               | `DDNS_NETWORK_CARD_IPV4`   | IPv4 使用的网卡键名                        |
| `network_card.ipv6`     | string | `""`                               | `DDNS_NETWORK_CARD_IPV6`   | IPv6 使用的网卡键名                        |
| `services.dnspod`       | bool   | `false`                            | `DDNS_SERVICE_DNSPOD`      | 启用 DNSPod                                |
| `services.alidns`       | bool   | `false`                            | `DDNS_SERVICE_ALIDNS`      | 启用阿里云 DNS                             |
| `services.cloudflare`   | bool   | `false`                            | `DDNS_SERVICE_CLOUDFLARE`  | 启用 Cloudflare                            |
| `services.huawei_cloud` | bool   | `false`                            | `DDNS_SERVICE_HUAWEI`      | 启用华为云 DNS                             |
| `enable_ipv6_fallback`  | bool   | `true`                             | `DDNS_IPV6_FALLBACK`       | 当首选网卡无 IPv6 全球单播地址时自动回落   |
| `check_cycle_minutes`   | int    | `0`                                | `DDNS_CHECK_CYCLE`         | 更新检查间隔（分钟），0 表示运行一次后退出 |

> 注意：将客户端安装为系统服务（`-I` / `service install`）时，`check_cycle_minutes`
> 必须大于 0，否则服务执行一次检查后就会立即退出。
>
> 某一轮检查如果没能成功交付（网络故障、token 被拒、服务商报错），会在下一轮重试，
> 而不是一直跳过直到 IP 再次变化。

**client.json 示例：**

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

| JSON 字段         | 环境变量               | 说明                             |
|-------------------|------------------------|----------------------------------|
| `id`              | `DDNS_DNSPOD_ID`       | DNSPod API ID                    |
| `token`           | `DDNS_DNSPOD_TOKEN`    | DNSPod API 令牌                  |
| `domain`          | `DDNS_DNSPOD_DOMAIN`   | 根域名，例如 `example.com`       |
| `sub_domain.a`    | `DDNS_DNSPOD_SUB_A`    | A 记录子域名，例如 `@` 或 `home` |
| `sub_domain.aaaa` | `DDNS_DNSPOD_SUB_AAAA` | AAAA 记录子域名                  |

---

## alidns.json

| JSON 字段           | 环境变量                | 说明                    |
|---------------------|-------------------------|-------------------------|
| `access_key_id`     | `DDNS_ALIDNS_AK_ID`     | 阿里云 AccessKey ID     |
| `access_key_secret` | `DDNS_ALIDNS_AK_SECRET` | 阿里云 AccessKey Secret |
| `domain`            | `DDNS_ALIDNS_DOMAIN`    | 根域名                  |
| `sub_domain.a`      | `DDNS_ALIDNS_SUB_A`     | A 记录子域名            |
| `sub_domain.aaaa`   | `DDNS_ALIDNS_SUB_AAAA`  | AAAA 记录子域名         |

---

## cloudflare.json

| JSON 字段     | 环境变量                      | 说明                                    |
|---------------|-------------------------------|-----------------------------------------|
| `zone_id`     | `DDNS_CLOUDFLARE_ZONE_ID`     | Cloudflare 区域 ID                      |
| `api_token`   | `DDNS_CLOUDFLARE_TOKEN`       | Cloudflare API 令牌                     |
| `domain.a`    | `DDNS_CLOUDFLARE_DOMAIN_A`    | A 记录完整域名，例如 `home.example.com` |
| `domain.aaaa` | `DDNS_CLOUDFLARE_DOMAIN_AAAA` | AAAA 记录完整域名                       |
| `proxied`     | `DDNS_CLOUDFLARE_PROXIED`     | 是否开启 Cloudflare 代理（橙色云朵）    |

---

## huaweicloud.json

| JSON 字段           | 环境变量                  | 说明                                     |
|---------------------|---------------------------|------------------------------------------|
| `access_key_id`     | `DDNS_HUAWEI_AK_ID`       | 华为云 Access Key ID                     |
| `secret_access_key` | `DDNS_HUAWEI_AK_SECRET`   | 华为云 Secret Access Key                 |
| `zone_name`         | `DDNS_HUAWEI_ZONE_NAME`   | 带尾部点的区域名，例如 `example.com.`    |
| `domain.a`          | `DDNS_HUAWEI_DOMAIN_A`    | A 记录完整域名，例如 `home.example.com.` |
| `domain.aaaa`       | `DDNS_HUAWEI_DOMAIN_AAAA` | AAAA 记录完整域名                        |

---

## server.json

| JSON 字段         | 环境变量                   | 默认值                            | 说明                                   |
|-------------------|----------------------------|-----------------------------------|----------------------------------------|
| `server_addr`     | `DDNS_SERVER_ADDR`         | `:10032`                          | 监听地址                               |
| `is_root_server`  | `DDNS_SERVER_IS_ROOT`      | `false`                           | 本机是否为根服务器（版本信息权威来源） |
| `root_server_url` | `DDNS_SERVER_ROOT_URL`     | `https://yzyweb.cn/ddns-watchdog` | 上游服务器地址（用于版本检查）         |
| `center_service`  | `DDNS_SERVER_CENTER`       | `false`                           | 启用中心代理接口                       |
| `route.get_ip`    | `DDNS_SERVER_ROUTE_GETIP`  | `/`                               | IP 回显接口路由                        |
| `route.center`    | `DDNS_SERVER_ROUTE_CENTER` | `/center`                         | 中心代理接口路由                       |
| `tls.enable`      | `DDNS_SERVER_TLS`          | `false`                           | 启用 TLS                               |
| `tls.cert_file`   | `DDNS_SERVER_TLS_CERT`     | `""`                              | TLS 证书文件路径                       |
| `tls.key_file`    | `DDNS_SERVER_TLS_KEY`      | `""`                              | TLS 密钥文件路径                       |

---

## services.json

启用中心代理时，服务器端使用的 DNS 服务商凭证。

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

授权客户端 token 通过中心代理更新特定 DNS 记录。

```json
{
    "<token>": {
        "enable": true,
        "description": "家庭服务器",
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

通过服务器命令行管理白名单条目：

```bash
# 添加或更新
./ddns-watchdog-server -a -t <token> -s dnspod -D example.com -A @ --AAAA @ -m "home"

# 删除
./ddns-watchdog-server -d -t <token>
```

服务器运行期间会即时感知该文件的变化，用这种方式添加或删除的 token 会立刻生效，
无需重启服务器。

`-a` 只写入你传入的字段，已有条目的其他字段保持不变，因此更新一条记录不会重置其他
记录。更新时 `-s`、`-D`、`-A`、`--AAAA`、`-m` 都可以省略。

center 如何解释 `-D`、`-A`、`--AAAA` 取决于服务商，因为各家寻址记录的方式不同：

| 服务商             | `-D`                                   | `-A` / `--AAAA`                          |
|--------------------|----------------------------------------|------------------------------------------|
| `dnspod`、`alidns` | 根域名，如 `example.com`               | 子域名，如 `@` 或 `home`                 |
| `cloudflare`       | 根域名                                 | 子域名；center 会拼成 `home.example.com` |
| `huaweicloud`      | **带末尾点的** 域名，如 `example.com.` | 完整记录名，如 `home.example.com.`       |

center 返回给客户端的一律是 JSON：token 不存在或被禁用返回 `403`，服务端未启用该服务商
返回 `403`，请求体无法解析返回 `400`，服务商拒绝更新返回 `502`。返回 `200` 表示请求的
记录都已更新或本来就正确。
