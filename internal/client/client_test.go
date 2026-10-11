package client

import (
	"ddns-watchdog/internal/common"
	"os"
	"path/filepath"
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
