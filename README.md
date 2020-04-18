# 动态域名解析
![](https://travis-ci.com/yzy613/ddns.svg?branch=master)


基于 Go 的 DDNS 客户端和服务端

## 客户端 用途
- 自动将域名解析到动态 IP（支持 IPv6）

## 客户端 用法
- `./ddns-client` 直接运行

- `./ddns-client -mt` 显示更多的提示

- `./ddns-client -f` 强制检查 DNS 解析记录

- `./ddns-client -version` 查看当前版本

- 加入计划任务定时执行

### 第一次使用？
- 初始化过程中会重启多次，请留意返回的信息

- 如果程序无提示直接结束，请添加 `-mt` 启动参数以显示更多的提示

#### DNSPod
- 请打开配置文件 `./conf/dnspod.json` 填入你的 `id, token, domain, sub_domain` 并重新启动

- 如果没有 `./conf/dnspod.json` 配置文件，请注意是否在 `./conf/client.json` 启用 `dnspod`

#### 万网（阿里云解析）
- 将在以后的不久支持

## 服务端 用途
- 返回 Json 格式的客户端 IP（支持 IPv6）

## 服务端 用法
- `./ddns-server -install` 安装服务

- 使用 `systemctl start ddns-server` 启动

- `./ddns-server -uninstall` 卸载服务

- `./ddns-server -version` 查看当前版本