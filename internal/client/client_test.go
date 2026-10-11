package client

import (
	"ddns-watchdog/internal/common"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clearClientEnv removes every DDNS_* override so a test only sees the config
// file (or its absence).
func clearClientEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"DDNS_API_URL_IPV4", "DDNS_API_URL_IPV6", "DDNS_API_URL_VERSION",
		"DDNS_CENTER_ENABLE", "DDNS_CENTER_URL", "DDNS_CENTER_TOKEN",
		"DDNS_ENABLE_IPV4", "DDNS_ENABLE_IPV6",
		"DDNS_NETWORK_CARD_ENABLE", "DDNS_NETWORK_CARD_IPV4", "DDNS_NETWORK_CARD_IPV6",
		"DDNS_SERVICE_DNSPOD", "DDNS_SERVICE_ALIDNS", "DDNS_SERVICE_CLOUDFLARE", "DDNS_SERVICE_HUAWEI",
		"DDNS_IPV6_FALLBACK", "DDNS_CHECK_CYCLE",
		"DDNS_DNSPOD_ID", "DDNS_DNSPOD_TOKEN", "DDNS_DNSPOD_DOMAIN", "DDNS_DNSPOD_SUB_A", "DDNS_DNSPOD_SUB_AAAA",
		"DDNS_ALIDNS_AK_ID", "DDNS_ALIDNS_AK_SECRET", "DDNS_ALIDNS_DOMAIN", "DDNS_ALIDNS_SUB_A", "DDNS_ALIDNS_SUB_AAAA",
		"DDNS_CLOUDFLARE_ZONE_ID", "DDNS_CLOUDFLARE_TOKEN", "DDNS_CLOUDFLARE_DOMAIN_A", "DDNS_CLOUDFLARE_DOMAIN_AAAA",
		"DDNS_CLOUDFLARE_PROXIED",
		"DDNS_HUAWEI_AK_ID", "DDNS_HUAWEI_AK_SECRET", "DDNS_HUAWEI_ZONE_NAME", "DDNS_HUAWEI_DOMAIN_A", "DDNS_HUAWEI_DOMAIN_AAAA",
	} {
		t.Setenv(k, "")
	}
}

// With no client.json at all only DDNS_ENABLE_IPV4/DDNS_SERVICE_DNSPOD are set,
// so every other field must fall back to the documented default - in particular
// enable_ipv6_fallback, which InitConf writes as true.
func TestLoadConfEnvOnlyUsesDefaults(t *testing.T) {
	clearClientEnv(t)
	t.Setenv("DDNS_ENABLE_IPV4", "true")
	t.Setenv("DDNS_SERVICE_DNSPOD", "true")

	origConfDir, origClient := ConfDir, Client
	defer func() { ConfDir, Client = origConfDir, origClient }()

	ConfDir = t.TempDir() // no client.json
	Client = client{}

	if err := Client.LoadConf(); err != nil {
		t.Fatalf("LoadConf without client.json: %v", err)
	}
	if !Client.Enable.IPv4 || Client.Enable.IPv6 {
		t.Errorf("enable = %+v, want ipv4 only", Client.Enable)
	}
	if !Client.EnableIPv6Fallback {
		t.Error("enable_ipv6_fallback = false, want true (documented default)")
	}
	if Client.APIUrl.IPv4 != common.DefaultAPIUrl {
		t.Errorf("api_url.ipv4 = %q, want %q", Client.APIUrl.IPv4, common.DefaultAPIUrl)
	}
	if Client.APIUrl.IPv6 != common.DefaultIPv6APIUrl {
		t.Errorf("api_url.ipv6 = %q, want %q", Client.APIUrl.IPv6, common.DefaultIPv6APIUrl)
	}
	if Client.CheckCycleMinutes != 0 {
		t.Errorf("check_cycle_minutes = %d, want 0", Client.CheckCycleMinutes)
	}
}

// A partial file must not be able to turn a documented default off by omission,
// and values it does set must win over the defaults.
func TestLoadConfPartialFileMergesDefaults(t *testing.T) {
	clearClientEnv(t)

	origConfDir, origClient := ConfDir, Client
	defer func() { ConfDir, Client = origConfDir, origClient }()

	ConfDir = t.TempDir()
	conf := `{"enable": {"ipv6": true}, "services": {"cloudflare": true}, "enable_ipv6_fallback": false}`
	if err := os.WriteFile(filepath.Join(ConfDir, ConfFilename), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	Client = client{}

	if err := Client.LoadConf(); err != nil {
		t.Fatalf("LoadConf with a partial client.json: %v", err)
	}
	if !Client.Enable.IPv6 || Client.Enable.IPv4 {
		t.Errorf("enable = %+v, want ipv6 only", Client.Enable)
	}
	if !Client.Services.Cloudflare {
		t.Error("services.cloudflare = false, want true from the file")
	}
	if Client.EnableIPv6Fallback {
		t.Error("enable_ipv6_fallback = true, want the explicit false from the file")
	}
	if Client.APIUrl.IPv4 != common.DefaultAPIUrl {
		t.Errorf("api_url.ipv4 = %q, want the default %q", Client.APIUrl.IPv4, common.DefaultAPIUrl)
	}
}

// A fresh, entirely unconfigured run must say which file to edit and how to
// create it.
func TestLoadConfMissingFileHint(t *testing.T) {
	clearClientEnv(t)

	origConfDir, origClient := ConfDir, Client
	defer func() { ConfDir, Client = origConfDir, origClient }()

	ConfDir = t.TempDir() // no client.json
	Client = client{}

	err := Client.LoadConf()
	if err == nil {
		t.Fatal("LoadConf succeeded without any IP type enabled")
	}
	for _, want := range []string{"no IP type enabled", ConfFilename, "-i 0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// mapString must never panic on a response shape the provider did not promise.
func TestMapString(t *testing.T) {
	obj := map[string]any{"str": "value", "num": 1, "null": nil}

	if got, ok := mapString(obj, "str"); !ok || got != "value" {
		t.Errorf("mapString(str) = %q, %v; want \"value\", true", got, ok)
	}
	for _, key := range []string{"num", "null", "absent"} {
		if got, ok := mapString(obj, key); ok {
			t.Errorf("mapString(%q) = %q, true; want false for a non-string field", key, got)
		}
	}
}

// A provider config file that is missing is reported as a missing file, not as a
// list of unset fields.
func TestProviderLoadConfReportsMissingFile(t *testing.T) {
	clearClientEnv(t)

	origConfDir, origDP := ConfDir, DP
	defer func() { ConfDir, DP = origConfDir, origDP }()

	ConfDir = t.TempDir() // no dnspod.json
	DP = DNSPod{}

	err := DP.LoadConf()
	if err == nil {
		t.Fatal("LoadConf accepted a missing dnspod.json")
	}
	if !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("error %q does not report the missing file", err)
	}
}

// ...but a provider configured entirely through environment variables must still
// load without the file, which is what the Docker deployment relies on.
func TestProviderLoadConfEnvOnly(t *testing.T) {
	clearClientEnv(t)
	t.Setenv("DDNS_DNSPOD_ID", "12345")
	t.Setenv("DDNS_DNSPOD_TOKEN", "abcdef0123456789")
	t.Setenv("DDNS_DNSPOD_DOMAIN", "example.com")
	t.Setenv("DDNS_DNSPOD_SUB_A", "home")

	origConfDir, origDP := ConfDir, DP
	defer func() { ConfDir, DP = origConfDir, origDP }()

	ConfDir = t.TempDir() // no dnspod.json
	DP = DNSPod{}

	if err := DP.LoadConf(); err != nil {
		t.Fatalf("LoadConf with environment-only config: %v", err)
	}
	if DP.ID != "12345" || DP.Domain != "example.com" || DP.SubDomain.A != "home" {
		t.Errorf("environment overrides were not applied: %+v", DP)
	}
}
