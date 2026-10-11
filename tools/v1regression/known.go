package main

// This file holds every difference between v1 and v2 that the harness accepts.
// It is the machine-readable form of the v1..v2 behavioral changelog.
//
// Two mechanisms exist:
//
//   - translations: pure wording changes. v2 translated v1's Chinese user-facing
//     text (including the placeholder comments written into the config templates)
//     into English. They are normalized away, so the rest of the text is still
//     compared strictly.
//   - intended: deliberate behavior changes. Each entry must carry a reason
//     explaining why v2 is allowed to differ; anything not listed here is
//     reported as a regression.
//
// Both lists are filled in only after inspecting a real difference and
// confirming it is not a bug.

var translations = []Translation{
	// --- init messages: "初始化 <path>" -> "initialized <path>" ---
	{V2: "initialized ", V1: "初始化 ", Doc: "init message translated"},

	// --- config template placeholder text (same keys, same schema) ---
	{V2: `"get from https://console.dnspod.cn/account/token/token"`,
		V1: `"在 https://console.dnspod.cn/account/token/token 获取"`, Doc: "dnspod template placeholder"},
	{V2: `"get from https://ram.console.aliyun.com/users"`,
		V1: `"在 https://ram.console.aliyun.com/users 获取"`, Doc: "alidns template placeholder"},
	{V2: `"get from https://dash.cloudflare.com/profile/api-tokens"`,
		V1: `"在 https://dash.cloudflare.com/profile/api-tokens 获取"`, Doc: "cloudflare template placeholder"},
	{V2: `"get from https://console.huaweicloud.com/iam/"`,
		V1: `"在 https://console.huaweicloud.com/iam/ 获取"`, Doc: "huaweicloud template placeholder"},
	{V2: `"zone ID from the right sidebar of your domain page"`,
		V1: `"在你域名页面的右下角有个区域 ID"`, Doc: "cloudflare zone id placeholder"},
	{V2: `"subdomain for A record"`, V1: `"A记录子域名"`, Doc: "subdomain placeholder"},
	{V2: `"subdomain for AAAA record"`, V1: `"AAAA记录子域名"`, Doc: "subdomain placeholder"},
	// Cloudflare and Huawei use full record names, so their A and AAAA
	// placeholders differ per file and cannot share one mapping.
	{V2: "\t\t\"a\": \"subdomain.example.com\",", V1: "\t\t\"a\": \"A记录子域名.example.com\",",
		Doc: "cloudflare full-name placeholder"},
	{V2: "\t\t\"aaaa\": \"subdomain.example.com\"", V1: "\t\t\"aaaa\": \"AAAA记录子域名.example.com\"",
		Doc: "cloudflare full-name placeholder"},
	{V2: "\t\t\"a\": \"subdomain.example.com.\",", V1: "\t\t\"a\": \"A记录子域名.example.com.\",",
		Doc: "huaweicloud full-name placeholder"},
	{V2: "\t\t\"aaaa\": \"subdomain.example.com.\"", V1: "\t\t\"aaaa\": \"AAAA记录子域名.example.com.\"",
		Doc: "huaweicloud full-name placeholder"},

	// --- flag help text ---
	{V2: "config directory (use quotes if path contains spaces)", V1: "指定配置文件目录 (目录有空格请放在双引号中间)", Doc: "flag help"},
	{V2: "force DNS record update regardless of IP change", V1: "强制检查 DNS 解析记录", Doc: "flag help"},
	{V2: "selectively initialize config files and exit; can be combined (e.g. 01)",
		V1: "有选择地初始化配置文件并退出，可以组合使用 (例 01)", Doc: "flag help"},
	// longest first: "uninstall service and exit" contains "install service and exit"
	{V2: "uninstall service and exit", V1: "卸载服务并退出", Doc: "flag help"},
	{V2: "install service and exit", V1: "安装服务并退出", Doc: "flag help"},
	{V2: "print network card info and exit", V1: "输出网卡信息并退出", Doc: "flag help"},
	{V2: "print version and check for updates, then exit", V1: "查看当前版本并检查更新后退出", Doc: "flag help"},

	// --- version output ---
	{V2: "current version: ", V1: "当前版本 ", Doc: "version tips"},
	{V2: "latest version: ", V1: "最新版本 ", Doc: "version tips"},
	{V2: "git commit: ", V1: "Git Commit: ", Doc: "version tips"},
	{V2: "build time: ", V1: "Build Time: ", Doc: "version tips"},
	// the two update hints end differently, so the whole line is matched; the
	// project URL inside it is normalized to <PROJECT_URL> first
	{V2: "new version available, visit <PROJECT_URL> to download",
		V1: "发现新版本，请前往 <PROJECT_URL> 下载", Doc: "version tips"},
	{V2: "check for updates manually at <PROJECT_URL>",
		V1: "需要手动检查更新，请前往 <PROJECT_URL> 查看", Doc: "version tips"},

	// --- version lookup failures ---
	{V2: "N/A (check network connection)", V1: "N/A (请检查网络连接)", Doc: "version lookup failure"},
	{V2: "N/A (invalid response)", V1: "N/A (数据包错误)", Doc: "version lookup failure"},
	{V2: "N/A (version info not found)", V1: "N/A (没有获取到版本信息)", Doc: "version lookup failure"},

	// --- messages that only appear in one place ---
	{V2: "unknown init option: 9", V1: "你初始化了一个寂寞", Doc: "unknown -i option message"},
	{V2: "the center returned status code 403", V1: "The status code returned by the center is 403",
		Doc: "center rejection wording"},
	{V2: "open <DIR>/network_card.json to select a network card and set it in <DIR>/client.json",
		V1:  "请打开 <DIR>/network_card.json 选择网卡填入 <DIR>/client.json 的 network_card",
		Doc: "network card hint"},
	{V2: "unexpected ipv4 format: ", V1: "获取到的 IPv4 格式错误，意外获取到了 ", Doc: "wrong address family"},
	{V2: "unexpected ipv6 format: ", V1: "获取到的 IPv6 格式错误，意外获取到了 ", Doc: "wrong address family"},
	{V2: "ipv4: selected network card does not exist", V1: "IPv4 选择了不存在的网卡", Doc: "missing interface"},
	{V2: "ipv6: selected network card does not exist", V1: "IPv6 选择了不存在的网卡", Doc: "missing interface"},

	// --- provider results ---
	{V2: " record updated to ", V1: " 已更新解析记录 ", Doc: "provider update message"},
	{V2: " record not found", V1: " 解析记录不存在", Doc: "provider lookup failure"},
	{V2: "authentication failed", V1: "身份认证似乎有问题", Doc: "cloudflare auth failure"},

	// --- whitelist CLI validation ---
	{V2: "token must be 16-127 characters", V1: "token 不符合要求", Doc: "token validation"},
	{V2: "token length must be between 16 and 127", V1: "生成 token 的长度不符合要求", Doc: "token validation"},
	{V2: "token message too long (max 32 chars)", V1: "token message 备注信息过长", Doc: "token validation"},
	{V2: "no token specified", V1: "未指定 token", Doc: "token validation"},
	{V2: "unsupported provider", V1: "不支持的服务供应商", Doc: "provider validation"},
	{V2: "no DNS record specified", V1: "没有指定解析记录", Doc: "record validation"},
	{V2: "no domain specified", V1: "没有指定需要操作的域名", Doc: "record validation"},
	{V2: "no provider specified", V1: "没有指定需要采用的服务供应商", Doc: "provider validation"},
}

var intended = []Intended{
	// Batch A rewrote the "nothing is configured" errors so they name the JSON
	// keys to set and, when the file is missing, how to create it.
	{
		Scenario: "nothing-enabled", Field: "stderr",
		V1Match: "启用需要使用的 IP 类型",
		V2Match: `no IP type enabled: set "enable"."ipv4"`,
		Why:     "round-2 batch A: config errors now name the keys to change (v2 adds detail, exit code unchanged)",
	},
	// v2 can be configured entirely through environment variables, which is what
	// makes the Docker deployment possible; v1 required a JSON file.
	{
		Scenario: "no-config-file", Field: "exit_code",
		V1Match: "1", V2Match: "0",
		Why: "v2 feature: environment-only configuration runs without client.json (v1 exits with an error)",
	},
	{
		Scenario: "no-config-file", Field: "stderr",
		V1Match: "请打开客户端配置文件", V2Match: "",
		Why: "v2 feature: environment-only configuration runs without client.json",
	},
	{
		Scenario: "no-config-file", Field: "requests",
		V2Match: "dnsapi.cn",
		Why:     "v2 feature: with env-only configuration the provider is actually called; v1 never gets that far",
	},
	{
		Scenario: "no-config-file", Field: "stderr",
		V1Match: "no such file or directory", V2Match: "",
		Why: "v2 feature: environment-only configuration needs no client.json",
	},
	// The provider config errors were rewritten in the same style as the client
	// ones: v2 names the fields instead of asking the user to check the file.
	{
		Scenario: "dnspod-no-credentials", Field: "stderr",
		V1Match: "请打开配置文件", V2Match: "check id, token, domain, sub_domain in",
		Why: "round-2 batch A: provider config errors name the fields to set (exit code unchanged)",
	},
	{
		Scenario: "cloudflare-no-credentials", Field: "stderr",
		V1Match: "请打开配置文件", V2Match: "check zone_id, api_token, domain in",
		Why: "round-2 batch A: provider config errors name the fields to set (exit code unchanged)",
	},
	// v1 silently ignored -D when the token already existed, which mixed a new
	// provider or subdomain with the old domain; v2 applies it.
	{
		Scenario: "whitelist-cli", Field: "file:whitelist.json",
		V1Match: `"domain": "example.com"`, V2Match: `"domain": "other.com"`,
		Why: "round-1 fix: -D now updates the domain of an existing whitelist entry (v1 ignored it)",
	},
	// v2 applies the documented defaults when server.json is absent, which is what
	// makes environment-only deployment possible; v1 refused to start.
	{
		Scenario: "startup-without-config", Field: "server_log",
		V1Match: "stat <DIR>/server.json: no such file", V2Match: "Work on <ADDR>",
		Why: "round-1 fix: v2 starts with the documented defaults when server.json is missing",
	},
	{
		Scenario: "startup-without-config", Field: "responses",
		V1Match: "SERVER NOT READY", V2Match: "GET / -> 200",
		Why: "round-1 fix: v2 serves the echo endpoint without a config file; v1 exits first",
	},
	{
		Scenario: "startup-without-config", Field: "responses",
		V1Match: "ERROR Get", V2Match: "",
		Why: "round-1 fix: v2 serves the echo endpoint without a config file; v1 is not listening",
	},
	// --- configuration compatibility (tools/v1regression compat scenarios) ---
	// An older v1 file predates the enable_ipv6_fallback key. v1 left it at the Go
	// zero value (false); v2 seeds the documented default (true) before parsing.
	{
		Scenario: "config-legacy-client", Field: "stdout",
		V1Match: "fallback=false", V2Match: "fallback=true",
		Why: "v2 applies the documented default enable_ipv6_fallback=true when an older v1 client.json predates the key (v1 left it false)",
	},

	// Batch B made every response carry a JSON body, including rejections, and
	// turned an unreadable request body into 400 instead of 500.
	{
		Scenario: "echo-wrong-method", Field: "responses",
		V1Match: "405 ct= body=", V2Match: "method not allowed, use GET",
		Why: "round-2 batch B: rejections now carry a JSON body (status code unchanged)",
	},
	{
		Scenario: "center-token-flows", Field: "responses",
		V1Match: "403 ct= body=", V2Match: "is not enabled on this server",
		Why: "round-2 batch B: rejections now carry a JSON body (status code unchanged)",
	},
	{
		Scenario: "center-token-flows", Field: "responses",
		V1Match: "403 ct= body=", V2Match: "token is disabled",
		Why: "round-2 batch B: rejections now carry a JSON body (status code unchanged)",
	},
	{
		Scenario: "center-token-flows", Field: "responses",
		V1Match: "403 ct= body=", V2Match: "invalid token",
		Why: "round-2 batch B: rejections now carry a JSON body (status code unchanged)",
	},
	{
		Scenario: "center-token-flows", Field: "responses",
		V1Match: "500 ct= body=", V2Match: "400 ct=application/json",
		Why: "round-2 batch B: an unreadable request body is a client error (400, with a message) instead of 500",
	},
	{
		Scenario: "center-token-flows", Field: "responses",
		V1Match: "405 ct= body=", V2Match: "method not allowed, use POST",
		Why: "round-2 batch B: rejections now carry a JSON body (status code unchanged)",
	},
	{
		Scenario: "no-config-file", Field: "requests",
		V2Match: "<ADDR> GET /",
		Why:     "v2 feature: with env-only configuration the IP API is queried at all; v1 stops before that",
	},
}
