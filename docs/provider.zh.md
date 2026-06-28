# 服务商配置指南

本文介绍如何获取各 DNS 服务商所需的凭证。

---

## DNSPod（腾讯云）

1. 登录 [DNSPod 控制台](https://console.dnspod.cn/account/token/token)
2. 新建 API 密钥，保存 **ID** 和 **Token**
3. 配置文件 `dnspod.json`：

```json
{
    "id": "<你的 ID>",
    "token": "<你的 Token>",
    "domain": "example.com",
    "sub_domain": {
        "a": "@",
        "aaaa": "@"
    }
}
```

或通过环境变量配置：

```
DDNS_SERVICE_DNSPOD=true
DDNS_DNSPOD_ID=<你的 ID>
DDNS_DNSPOD_TOKEN=<你的 Token>
DDNS_DNSPOD_DOMAIN=example.com
DDNS_DNSPOD_SUB_A=@
DDNS_DNSPOD_SUB_AAAA=@
```

---

## 阿里云 DNS

1. 登录 [RAM 控制台](https://ram.console.aliyun.com/users)
2. 创建 RAM 用户并授予 `AliyunDNSFullAccess` 权限
3. 为该用户创建 AccessKey，保存 **AccessKey ID** 和 **AccessKey Secret**
4. 配置文件 `alidns.json`：

```json
{
    "access_key_id": "<你的 AK ID>",
    "access_key_secret": "<你的 AK Secret>",
    "domain": "example.com",
    "sub_domain": {
        "a": "@",
        "aaaa": "@"
    }
}
```

或通过环境变量配置：

```
DDNS_SERVICE_ALIDNS=true
DDNS_ALIDNS_AK_ID=<你的 AK ID>
DDNS_ALIDNS_AK_SECRET=<你的 AK Secret>
DDNS_ALIDNS_DOMAIN=example.com
DDNS_ALIDNS_SUB_A=@
DDNS_ALIDNS_SUB_AAAA=@
```

---

## Cloudflare

1. 登录 [Cloudflare 控制台](https://dash.cloudflare.com)
2. 进入你的域名页面，从右侧栏复制 **区域 ID（Zone ID）**
3. 前往 [API 令牌](https://dash.cloudflare.com/profile/api-tokens)，创建一个对目标区域有 **编辑 DNS** 权限的令牌
4. 配置文件 `cloudflare.json`：

```json
{
    "zone_id": "<你的 Zone ID>",
    "api_token": "<你的 API 令牌>",
    "domain": {
        "a": "home.example.com",
        "aaaa": "home.example.com"
    },
    "proxied": false
}
```

或通过环境变量配置：

```
DDNS_SERVICE_CLOUDFLARE=true
DDNS_CLOUDFLARE_ZONE_ID=<你的 Zone ID>
DDNS_CLOUDFLARE_TOKEN=<你的 API 令牌>
DDNS_CLOUDFLARE_DOMAIN_A=home.example.com
DDNS_CLOUDFLARE_DOMAIN_AAAA=home.example.com
DDNS_CLOUDFLARE_PROXIED=false
```

> 注意：Cloudflare 的 `domain.a` 和 `domain.aaaa` 填**完整域名**（不是子域名），例如 `home.example.com`。

---

## 华为云 DNS

1. 登录 [IAM 控制台](https://console.huaweicloud.com/iam/)
2. 创建 IAM 用户并授予 DNS 管理员权限
3. 创建 AccessKey，保存 **访问密钥 ID** 和 **秘密访问密钥**
4. 配置文件 `huaweicloud.json`：

```json
{
    "access_key_id": "<你的 AK ID>",
    "secret_access_key": "<你的 SK>",
    "zone_name": "example.com.",
    "domain": {
        "a": "home.example.com.",
        "aaaa": "home.example.com."
    }
}
```

或通过环境变量配置：

```
DDNS_SERVICE_HUAWEI=true
DDNS_HUAWEI_AK_ID=<你的 AK ID>
DDNS_HUAWEI_AK_SECRET=<你的 SK>
DDNS_HUAWEI_ZONE_NAME=example.com.
DDNS_HUAWEI_DOMAIN_A=home.example.com.
DDNS_HUAWEI_DOMAIN_AAAA=home.example.com.
```

> 注意：华为云的区域名称（zone_name）和域名（domain）均需以**英文句点（.）**结尾，例如 `example.com.`。
