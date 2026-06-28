# Provider Setup Guide

This guide explains how to obtain credentials for each supported DNS provider.

---

## DNSPod (Tencent Cloud)

1. Log in to [DNSPod Console](https://console.dnspod.cn/account/token/token)
2. Create a new API key — save the **ID** and **Token**
3. Your `dnspod.json`:

```json
{
    "id": "<your-id>",
    "token": "<your-token>",
    "domain": "example.com",
    "sub_domain": {
        "a": "@",
        "aaaa": "@"
    }
}
```

Or with environment variables:

```
DDNS_SERVICE_DNSPOD=true
DDNS_DNSPOD_ID=<your-id>
DDNS_DNSPOD_TOKEN=<your-token>
DDNS_DNSPOD_DOMAIN=example.com
DDNS_DNSPOD_SUB_A=@
DDNS_DNSPOD_SUB_AAAA=@
```

---

## AliDNS (Alibaba Cloud)

1. Log in to [RAM Console](https://ram.console.aliyun.com/users)
2. Create a RAM user and grant `AliyunDNSFullAccess` permission
3. Create an AccessKey for that user — save the **AccessKey ID** and **AccessKey Secret**
4. Your `alidns.json`:

```json
{
    "access_key_id": "<your-ak-id>",
    "access_key_secret": "<your-ak-secret>",
    "domain": "example.com",
    "sub_domain": {
        "a": "@",
        "aaaa": "@"
    }
}
```

Or with environment variables:

```
DDNS_SERVICE_ALIDNS=true
DDNS_ALIDNS_AK_ID=<your-ak-id>
DDNS_ALIDNS_AK_SECRET=<your-ak-secret>
DDNS_ALIDNS_DOMAIN=example.com
DDNS_ALIDNS_SUB_A=@
DDNS_ALIDNS_SUB_AAAA=@
```

---

## Cloudflare

1. Log in to [Cloudflare Dashboard](https://dash.cloudflare.com)
2. Go to your domain → copy the **Zone ID** from the right sidebar
3. Go to [API Tokens](https://dash.cloudflare.com/profile/api-tokens) → Create token with **Edit zone DNS** permission for your zone
4. Your `cloudflare.json`:

```json
{
    "zone_id": "<your-zone-id>",
    "api_token": "<your-api-token>",
    "domain": {
        "a": "home.example.com",
        "aaaa": "home.example.com"
    },
    "proxied": false
}
```

Or with environment variables:

```
DDNS_SERVICE_CLOUDFLARE=true
DDNS_CLOUDFLARE_ZONE_ID=<your-zone-id>
DDNS_CLOUDFLARE_TOKEN=<your-api-token>
DDNS_CLOUDFLARE_DOMAIN_A=home.example.com
DDNS_CLOUDFLARE_DOMAIN_AAAA=home.example.com
DDNS_CLOUDFLARE_PROXIED=false
```

> Note: For Cloudflare, `domain.a` and `domain.aaaa` are **full domain names** (not subdomains), e.g. `home.example.com`.

---

## Huawei Cloud DNS

1. Log in to [IAM Console](https://console.huaweicloud.com/iam/)
2. Create an IAM user and grant DNS admin permission
3. Create an AccessKey — save the **Access Key ID** and **Secret Access Key**
4. Your `huaweicloud.json`:

```json
{
    "access_key_id": "<your-ak-id>",
    "secret_access_key": "<your-sk>",
    "zone_name": "example.com.",
    "domain": {
        "a": "home.example.com.",
        "aaaa": "home.example.com."
    }
}
```

Or with environment variables:

```
DDNS_SERVICE_HUAWEI=true
DDNS_HUAWEI_AK_ID=<your-ak-id>
DDNS_HUAWEI_AK_SECRET=<your-sk>
DDNS_HUAWEI_ZONE_NAME=example.com.
DDNS_HUAWEI_DOMAIN_A=home.example.com.
DDNS_HUAWEI_DOMAIN_AAAA=home.example.com.
```

> Note: Huawei Cloud zone names and domain names require a **trailing dot** (e.g. `example.com.`).
