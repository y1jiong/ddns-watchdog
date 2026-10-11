package server

import (
	"ddns-watchdog/internal/common"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// clearServerEnv removes every DDNS_SERVER_* override so a test only sees the
// config file (or its absence).
func clearServerEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"DDNS_SERVER_ADDR", "DDNS_SERVER_IS_ROOT", "DDNS_SERVER_ROOT_URL",
		"DDNS_SERVER_CENTER", "DDNS_SERVER_ROUTE_GETIP", "DDNS_SERVER_ROUTE_CENTER",
		"DDNS_SERVER_TLS", "DDNS_SERVER_TLS_CERT", "DDNS_SERVER_TLS_KEY",
	} {
		t.Setenv(k, "")
	}
}

// An env-only deployment has no server.json at all; the documented defaults must
// still apply, otherwise the server binds :80 and http.HandleFunc("") panics.
func TestLoadConfEnvOnlyUsesDefaults(t *testing.T) {
	clearServerEnv(t)

	useTempConf(t) // no server.json

	if err := Srv.LoadConf(); err != nil {
		t.Fatalf("LoadConf without server.json: %v", err)
	}
	if Srv.ServerAddr != defaultServerAddr {
		t.Errorf("server_addr = %q, want %q", Srv.ServerAddr, defaultServerAddr)
	}
	if Srv.Route.GetIP != defaultRouteGetIP {
		t.Errorf("route.get_ip = %q, want %q", Srv.Route.GetIP, defaultRouteGetIP)
	}
	if Srv.Route.Center != defaultRouteCenter {
		t.Errorf("route.center = %q, want %q", Srv.Route.Center, defaultRouteCenter)
	}

	// the routes must be usable patterns - an empty one panics inside ServeMux
	mux := http.NewServeMux()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("registering config routes panics: %v", r)
		}
	}()
	mux.HandleFunc(Srv.Route.GetIP, RespGetIPReq)
}

func TestLoadConfRejectsDuplicateRoutes(t *testing.T) {
	clearServerEnv(t)

	useTempConf(t)
	conf := `{"center_service": true, "route": {"get_ip": "/same", "center": "/same"}}`
	if err := os.WriteFile(filepath.Join(ConfDir, ConfFilename), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Srv.LoadConf(); err == nil {
		t.Fatal("LoadConf accepted identical route.get_ip and route.center, which panics in ServeMux")
	}
}

func TestLoadConfRejectsIncompleteTLS(t *testing.T) {
	clearServerEnv(t)

	useTempConf(t)
	if err := os.WriteFile(filepath.Join(ConfDir, ConfFilename), []byte(`{"tls": {"enable": true}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Srv.LoadConf(); err == nil {
		t.Fatal("LoadConf accepted tls.enable without cert_file/key_file")
	}
}

// Updating an existing token must apply -D, not silently keep the stored domain.
func TestAddToWhitelistUpdateAppliesDomain(t *testing.T) {
	useTempConf(t)

	if _, err := InitWhitelist(); err != nil {
		t.Fatalf("InitWhitelist: %v", err)
	}

	const token = "0123456789abcdef"
	if _, err := AddToWhitelist(token, "home", common.DNSPod, "example.com", "a", "aaaa"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	status, err := AddToWhitelist(token, "", "", "other.com", "b", "")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if status != UpdateSign {
		t.Errorf("status = %q, want %q", status, UpdateSign)
	}

	got, ok := getWhitelistEntry(token)
	if !ok {
		t.Fatal("token disappeared from the whitelist")
	}
	if got.DomainRecord.Domain != "other.com" {
		t.Errorf("domain = %q, want other.com (-D is ignored on update)", got.DomainRecord.Domain)
	}
	if got.DomainRecord.Subdomain.A != "b" {
		t.Errorf("subdomain.a = %q, want b", got.DomainRecord.Subdomain.A)
	}
	if got.DomainRecord.Subdomain.AAAA != "aaaa" {
		t.Errorf("subdomain.aaaa = %q, want it left unchanged when --AAAA is omitted", got.DomainRecord.Subdomain.AAAA)
	}
	if got.Service != common.DNSPod {
		t.Errorf("service = %q, want it left unchanged when -s is omitted", got.Service)
	}
}
