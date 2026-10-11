package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Scenario is one comparable behavior of the two revisions.
type Scenario struct {
	Group string
	Name  string
	Doc   string
	Run   func(t *Target) (*Observation, error)

	// NewInV2 marks a scenario that exercises a capability v1 does not have at
	// all (for example the "service" subcommand). Every difference it reports is
	// then expected, and Why explains it. Use it only when the whole comparison,
	// not an individual line, is a deliberate addition.
	NewInV2 bool
	Why     string
}

func (s Scenario) String() string { return s.Group + "/" + s.Name }

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func prettyJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "\t")
	must(err)
	return string(b)
}

func merge(dst map[string]any, src map[string]any) map[string]any {
	for k, v := range src {
		if sub, ok := v.(map[string]any); ok {
			if cur, ok := dst[k].(map[string]any); ok {
				merge(cur, sub)
				continue
			}
		}
		dst[k] = v
	}
	return dst
}

// clientConf mirrors the documented client.json schema.
func clientConf(over map[string]any) string {
	conf := map[string]any{
		"api_url":              map[string]any{"ipv4": "", "ipv6": "", "version": ""},
		"center":               map[string]any{"api_url": "", "token": "", "enable": false},
		"enable":               map[string]any{"ipv4": false, "ipv6": false},
		"network_card":         map[string]any{"enable": false, "ipv4": "", "ipv6": ""},
		"services":             map[string]any{"dnspod": false, "alidns": false, "cloudflare": false, "huawei_cloud": false},
		"enable_ipv6_fallback": true,
		"check_cycle_minutes":  0,
	}
	return prettyJSON(merge(conf, over))
}

func serverConf(over map[string]any) string {
	conf := map[string]any{
		"server_addr":     "",
		"is_root_server":  true,
		"root_server_url": "",
		"center_service":  false,
		"route":           map[string]any{"get_ip": "/", "center": "/center"},
		"tls":             map[string]any{"enable": false, "cert_file": "", "key_file": ""},
	}
	return prettyJSON(merge(conf, over))
}

func dnspodConf(over map[string]any) string {
	conf := map[string]any{
		"id": "12345", "token": "abcdef0123456789", "domain": "example.com",
		"sub_domain": map[string]any{"a": "home", "aaaa": "home"},
	}
	return prettyJSON(merge(conf, over))
}

func cloudflareConf(over map[string]any) string {
	conf := map[string]any{
		"zone_id": "zone123", "api_token": "cf-token",
		"domain":  map[string]any{"a": "home.example.com", "aaaa": "home.example.com"},
		"proxied": false,
	}
	return prettyJSON(merge(conf, over))
}

// harnessFiles are produced by the harness rather than the binary under test, so
// their content is random per run and must not be compared.
var harnessFiles = map[string]bool{
	"ca.pem":     true,
	"server.crt": true,
	"server.key": true,
}

func collectFiles(dir string) map[string]string {
	files := map[string]string{}
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || harnessFiles[d.Name()] {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		files[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	return files
}

// observe turns a client run into an observation.
func observe(t *Target, res *CmdResult) *Observation {
	return &Observation{
		ExitCode: res.ExitCode,
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
		Files:    collectFiles(t.Dir),
	}
}

// startServer launches the server and waits for it to accept requests.
func startServer(t *Target, port int, obs *Observation) (*Proc, error) {
	proc, err := t.Start("server", "-c", t.Dir)
	if err != nil {
		return nil, err
	}
	if err := waitForHTTP(fmt.Sprintf("http://127.0.0.1:%d/", port), 5*time.Second); err != nil {
		obs.Responses = append(obs.Responses, "SERVER NOT READY: "+err.Error())
	}
	return proc, nil
}

// ---------------------------------------------------------------------------
// scenarios: CLI and configuration templates
// ---------------------------------------------------------------------------

var cliScenarios = []Scenario{
	{
		Group: "cli", Name: "init-client-templates",
		Doc: "client -i 01234 writes exactly the same config templates as v1",
		Run: func(t *Target) (*Observation, error) {
			res := t.Run("client", "-i", "01234", "-c", t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "cli", Name: "init-server-templates",
		Doc: "server -i 012 writes exactly the same config templates as v1",
		Run: func(t *Target) (*Observation, error) {
			res := t.Run("server", "-i", "012", "-c", t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "cli", Name: "init-invalid-option",
		Doc: "an unknown -i character is rejected with the same exit code",
		Run: func(t *Target) (*Observation, error) {
			res := t.Run("client", "-i", "9", "-c", t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "cli", Name: "network-cards",
		Doc: "client -n lists the same network cards and addresses",
		Run: func(t *Target) (*Observation, error) {
			res := t.Run("client", "-n", "-c", t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "cli", Name: "unknown-flag",
		Doc: "an unknown flag fails the same way",
		Run: func(t *Target) (*Observation, error) {
			res := t.Run("client", "-Z", "-c", t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "cli", Name: "relative-conf-dir",
		Doc: "a relative -c directory resolves against the working directory",
		Run: func(t *Target) (*Observation, error) {
			res := t.RunDir(t.Dir, "client", "-i", "0", "-c", "sub")
			return observe(t, res), nil
		},
	},
	{
		Group: "cli", Name: "version-check-offline",
		Doc: "an unreachable version API produces the same fallback text",
		Run: func(t *Target) (*Observation, error) {
			// nothing listens on port 1: the lookup fails immediately
			t.WriteFile("client.json", clientConf(map[string]any{
				"api_url":  map[string]any{"version": "http://127.0.0.1:1/"},
				"enable":   map[string]any{"ipv4": true},
				"services": map[string]any{"dnspod": true},
			}))
			t.WriteFile("dnspod.json", dnspodConf(nil))

			res := t.Run("client", "-V", "-c", t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "cli", Name: "version-check",
		Doc: "client -V prints the same version information for the same API reply",
		Run: func(t *Target) (*Observation, error) {
			api := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
				fmt.Fprint(w, `{"ip":"203.0.113.7","latest_version":"9.9.9"}`)
			})
			defer api.Close()

			t.WriteFile("client.json", clientConf(map[string]any{
				"api_url":  map[string]any{"version": api.URL},
				"enable":   map[string]any{"ipv4": true},
				"services": map[string]any{"dnspod": true},
			}))
			t.WriteFile("dnspod.json", dnspodConf(nil))

			res := t.Run("client", "-V", "-c", t.Dir)
			obs := observe(t, res)
			obs.Requests = api.Requests()
			return obs, nil
		},
	},
}

// ---------------------------------------------------------------------------
// scenarios: configuration compatibility (v1 files on v2)
// ---------------------------------------------------------------------------

// compatScenarios answer "can a v1 configuration directory be used unchanged?".
// They load the same files with both revisions' own code through tmp_probe and
// compare every loaded field, so a renamed, dropped or newly required key shows up.
var compatScenarios = []Scenario{
	{
		Group: "compat", Name: "config-identical",
		Doc: "a complete, current config directory loads field-for-field identically",
		Run: func(t *Target) (*Observation, error) {
			writeCompleteConf(t, clientConf(map[string]any{
				"api_url":              map[string]any{"ipv4": "https://ip.example.com", "ipv6": "https://ip6.example.com", "version": "https://ver.example.com"},
				"center":               map[string]any{"api_url": "https://center.example.com", "token": "0123456789abcdef", "enable": true},
				"enable":               map[string]any{"ipv4": true, "ipv6": true},
				"network_card":         map[string]any{"enable": true, "ipv4": "en0 5", "ipv6": "en0 1"},
				"services":             map[string]any{"dnspod": true, "alidns": true, "cloudflare": true, "huawei_cloud": true},
				"enable_ipv6_fallback": true,
				"check_cycle_minutes":  15,
			}))
			res := t.Probe(t.Dir, t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "compat", Name: "config-legacy-client",
		Doc: "an older v1 client.json without enable_ipv6_fallback still loads",
		Run: func(t *Target) (*Observation, error) {
			// the shape an older v1 shipped: no enable_ipv6_fallback key
			writeCompleteConf(t, `{
	"api_url": {"ipv4": "https://ip.example.com", "ipv6": "https://ip6.example.com", "version": "https://ver.example.com"},
	"center": {"api_url": "", "token": "", "enable": false},
	"enable": {"ipv4": true, "ipv6": true},
	"network_card": {"enable": true, "ipv4": "en0 5", "ipv6": "en0 1"},
	"services": {"dnspod": true, "alidns": false, "cloudflare": false, "huawei_cloud": false},
	"check_cycle_minutes": 0
}`)
			res := t.Probe(t.Dir, t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "compat", Name: "config-partial",
		Doc: "absent provider files are reported as missing files by both revisions",
		Run: func(t *Target) (*Observation, error) {
			t.WriteFile("client.json", clientConf(map[string]any{
				"enable":   map[string]any{"ipv4": true},
				"services": map[string]any{"dnspod": true},
			}))
			t.WriteFile("dnspod.json", dnspodConf(nil))
			// server.json is present so this scenario isolates the provider files;
			// the "server.json is absent" behavior has its own scenario
			// (server/startup-without-config).
			t.WriteFile("server.json", serverConf(map[string]any{
				"server_addr":     ":10032",
				"is_root_server":  true,
				"root_server_url": "https://root.example.com",
			}))

			res := t.Probe(t.Dir, t.Dir)
			return observe(t, res), nil
		},
	},
}

// writeCompleteConf writes every config file a deployment can have, so a
// compatibility scenario compares the configuration itself and not the
// missing-file handling.
func writeCompleteConf(t *Target, clientJSON string) {
	t.WriteFile("client.json", clientJSON)
	t.WriteFile("dnspod.json", dnspodConf(nil))
	t.WriteFile("alidns.json", prettyJSON(map[string]any{
		"access_key_id": "ak", "access_key_secret": "sk", "domain": "example.com",
		"sub_domain": map[string]any{"a": "home", "aaaa": "home"},
	}))
	t.WriteFile("cloudflare.json", cloudflareConf(nil))
	t.WriteFile("huaweicloud.json", prettyJSON(map[string]any{
		"access_key_id": "ak", "secret_access_key": "sk", "zone_name": "example.com.",
		"domain": map[string]any{"a": "home.example.com.", "aaaa": "home.example.com."},
	}))
	t.WriteFile("server.json", serverConf(map[string]any{
		"server_addr":     ":10032",
		"is_root_server":  false,
		"root_server_url": "https://root.example.com",
		"center_service":  true,
	}))
	t.WriteFile("services.json", prettyJSON(map[string]any{
		"dnspod":       map[string]any{"enable": true, "id": "12345", "token": "abcdef0123456789"},
		"alidns":       map[string]any{"enable": false, "access_key_id": "", "access_key_secret": ""},
		"cloudflare":   map[string]any{"enable": false, "zone_id": "", "api_token": ""},
		"huawei_cloud": map[string]any{"enable": true, "access_key_id": "ak", "secret_access_key": "sk"},
	}))
	t.WriteFile("whitelist.json", prettyJSON(map[string]any{
		"0123456789abcdef": map[string]any{
			"enable": true, "description": "home", "service": "dnspod",
			"domain_record": map[string]any{"domain": "example.com", "subdomain": map[string]any{"a": "home", "aaaa": "home"}},
		},
		"fedcba9876543210": map[string]any{
			"enable": false, "description": "off", "service": "cloudflare",
			"domain_record": map[string]any{"domain": "example.com", "subdomain": map[string]any{"a": "cf", "aaaa": "cf"}},
		},
	}))
}

// ---------------------------------------------------------------------------
// scenarios: client update paths
// ---------------------------------------------------------------------------

var clientScenarios = []Scenario{
	{
		Group: "client", Name: "center-ipv4",
		Doc: "center mode posts the same request body for an IPv4 update",
		Run: func(t *Target) (*Observation, error) {
			ipAPI := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
				fmt.Fprint(w, `{"ip":"203.0.113.7"}`)
			})
			defer ipAPI.Close()
			center := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
				fmt.Fprint(w, `{"message":"home.example.com updated"}`)
			})
			defer center.Close()

			t.WriteFile("client.json", clientConf(map[string]any{
				"api_url": map[string]any{"ipv4": ipAPI.URL},
				"center":  map[string]any{"api_url": center.URL, "token": "0123456789abcdef", "enable": true},
				"enable":  map[string]any{"ipv4": true},
			}))

			res := t.Run("client", "-c", t.Dir)
			obs := observe(t, res)
			obs.Requests = append(ipAPI.Requests(), center.Requests()...)
			return obs, nil
		},
	},
	{
		Group: "client", Name: "center-ipv6",
		Doc: "center mode posts the same request body for an IPv6 update",
		Run: func(t *Target) (*Observation, error) {
			ipAPI := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
				fmt.Fprint(w, `{"ip":"2001:db8::1"}`)
			})
			defer ipAPI.Close()
			center := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
				fmt.Fprint(w, `{"message":"ok"}`)
			})
			defer center.Close()

			t.WriteFile("client.json", clientConf(map[string]any{
				"api_url": map[string]any{"ipv6": ipAPI.URL},
				"center":  map[string]any{"api_url": center.URL, "token": "0123456789abcdef", "enable": true},
				"enable":  map[string]any{"ipv6": true},
			}))

			res := t.Run("client", "-c", t.Dir)
			obs := observe(t, res)
			obs.Requests = append(ipAPI.Requests(), center.Requests()...)
			return obs, nil
		},
	},
	{
		Group: "client", Name: "center-forbidden",
		Doc: "a rejected center update is reported without crashing",
		Run: func(t *Target) (*Observation, error) {
			ipAPI := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
				fmt.Fprint(w, `{"ip":"203.0.113.7"}`)
			})
			defer ipAPI.Close()
			center := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"message":"invalid token"}`)
			})
			defer center.Close()

			t.WriteFile("client.json", clientConf(map[string]any{
				"api_url": map[string]any{"ipv4": ipAPI.URL},
				"center":  map[string]any{"api_url": center.URL, "token": "0123456789abcdef", "enable": true},
				"enable":  map[string]any{"ipv4": true},
			}))

			res := t.Run("client", "-c", t.Dir)
			obs := observe(t, res)
			obs.Requests = append(ipAPI.Requests(), center.Requests()...)
			return obs, nil
		},
	},
	{
		Group: "client", Name: "ip-api-invalid-response",
		Doc: "a broken IP API reply is reported the same way",
		Run: func(t *Target) (*Observation, error) {
			ipAPI := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
				fmt.Fprint(w, `<html>not json</html>`)
			})
			defer ipAPI.Close()

			t.WriteFile("client.json", clientConf(map[string]any{
				"api_url":  map[string]any{"ipv4": ipAPI.URL},
				"enable":   map[string]any{"ipv4": true},
				"services": map[string]any{"dnspod": true},
			}))
			t.WriteFile("dnspod.json", dnspodConf(nil))

			res := t.Run("client", "-c", t.Dir)
			obs := observe(t, res)
			obs.Requests = ipAPI.Requests()
			return obs, nil
		},
	},
	{
		Group: "client", Name: "nothing-enabled",
		Doc: "a config that enables nothing is rejected",
		Run: func(t *Target) (*Observation, error) {
			t.WriteFile("client.json", clientConf(nil))
			res := t.Run("client", "-c", t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "client", Name: "no-config-file",
		Doc: "v2 can run from environment variables alone; v1 cannot",
		Run: func(t *Target) (*Observation, error) {
			ipAPI := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
				fmt.Fprint(w, `{"ip":"203.0.113.7"}`)
			})
			defer ipAPI.Close()

			proxy, err := NewProxy(func(host, method, path string, body []byte) Response {
				return Response{Status: 200, Body: `{"status":{"code":"1","message":"ok"},"records":[{"name":"home","type":"A","id":"11","value":"203.0.113.7","line_id":"0"}]}`}
			})
			if err != nil {
				return nil, err
			}
			defer proxy.Close()
			caFile := filepath.Join(t.Dir, "ca.pem")
			must(proxy.WriteCA(caFile))

			t.ExtraEnv = append(t.ExtraEnv,
				"DDNS_ENABLE_IPV4=true",
				"DDNS_SERVICE_DNSPOD=true",
				"DDNS_API_URL_IPV4="+ipAPI.URL,
				"DDNS_DNSPOD_ID=12345",
				"DDNS_DNSPOD_TOKEN=abcdef0123456789",
				"DDNS_DNSPOD_DOMAIN=example.com",
				"DDNS_DNSPOD_SUB_A=home",
			)
			t.ExtraEnv = append(t.ExtraEnv, proxy.Env(caFile)...)

			res := t.Run("client", "-c", t.Dir)
			obs := observe(t, res)
			obs.Requests = append(ipAPI.Requests(), proxy.Requests()...)
			return obs, nil
		},
	},
	{
		Group: "client", Name: "network-card-unconfigured",
		Doc: "network card mode dumps the interface list and asks for a selection",
		Run: func(t *Target) (*Observation, error) {
			t.WriteFile("client.json", clientConf(map[string]any{
				"enable":       map[string]any{"ipv4": true},
				"network_card": map[string]any{"enable": true, "ipv4": "", "ipv6": ""},
				"services":     map[string]any{"dnspod": true},
			}))
			t.WriteFile("dnspod.json", dnspodConf(nil))

			res := t.Run("client", "-c", t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "client", Name: "network-card-ipv4",
		Doc: "network card mode reads the interface address and reports it to the center",
		Run: func(t *Target) (*Observation, error) {
			key, _ := ipv4InterfaceKey(t.Run("client", "-n", "-c", t.Dir).Stdout)
			if key == "" {
				return &Observation{Responses: []string{"SKIP: no IPv4 interface on this host"}}, nil
			}

			center := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
				fmt.Fprint(w, `{"message":"ok"}`)
			})
			defer center.Close()

			t.WriteFile("client.json", clientConf(map[string]any{
				"enable":       map[string]any{"ipv4": true},
				"network_card": map[string]any{"enable": true, "ipv4": key, "ipv6": ""},
				"center":       map[string]any{"api_url": center.URL, "token": "0123456789abcdef", "enable": true},
			}))

			res := t.Run("client", "-c", t.Dir)
			obs := observe(t, res)
			obs.Requests = center.Requests()
			return obs, nil
		},
	},
	{
		Group: "client", Name: "network-card-family-mismatch",
		Doc: "an IPv4 interface key used for IPv6 is rejected the same way",
		Run: func(t *Target) (*Observation, error) {
			key, _ := ipv4InterfaceKey(t.Run("client", "-n", "-c", t.Dir).Stdout)
			if key == "" {
				return &Observation{Responses: []string{"SKIP: no IPv4 interface on this host"}}, nil
			}

			t.WriteFile("client.json", clientConf(map[string]any{
				"enable":       map[string]any{"ipv6": true},
				"network_card": map[string]any{"enable": true, "ipv4": "", "ipv6": key},
				"services":     map[string]any{"dnspod": true},
			}))
			t.WriteFile("dnspod.json", dnspodConf(nil))

			res := t.Run("client", "-c", t.Dir)
			return observe(t, res), nil
		},
	},
}

// ipv4InterfaceKey picks an interface key from the "client -n" output whose
// address is an IPv4, preferring a non-loopback one, so the scenario does not
// depend on which address families this host happens to have.
func ipv4InterfaceKey(out string) (key, value string) {
	lines := strings.Split(out, "\n")
	var fallbackKey, fallbackValue string

	for i := 0; i+1 < len(lines); i++ {
		if !strings.HasPrefix(lines[i+1], "\t") {
			continue
		}
		k := strings.TrimSpace(lines[i])
		v := strings.TrimSpace(lines[i+1])
		ip := net.ParseIP(v)
		if ip == nil || ip.To4() == nil {
			continue
		}
		if ip.IsLoopback() {
			if fallbackKey == "" {
				fallbackKey, fallbackValue = k, v
			}
			continue
		}
		return k, v
	}
	return fallbackKey, fallbackValue
}

// ---------------------------------------------------------------------------
// scenarios: DNS providers (through the TLS interception proxy)
// ---------------------------------------------------------------------------

var providerScenarios = []Scenario{
	{
		Group: "provider", Name: "dnspod-record-unchanged",
		Doc: "an unchanged DNSPod record is not modified",
		Run: func(t *Target) (*Observation, error) {
			obs, proxy, err := dnspodRun(t, `{"status":{"code":"1","message":"ok"},"records":[{"name":"home","type":"A","id":"11","value":"203.0.113.7","line_id":"0"}]}`, nil)
			if err != nil {
				return nil, err
			}
			defer proxy.Close()
			return obs, nil
		},
	},
	{
		Group: "provider", Name: "dnspod-record-changed",
		Doc: "a changed DNSPod record is modified with an identical request",
		Run: func(t *Target) (*Observation, error) {
			obs, proxy, err := dnspodRun(t, `{"status":{"code":"1","message":"ok"},"records":[{"name":"home","type":"A","id":"11","value":"198.51.100.1","line_id":"0"}]}`, nil)
			if err != nil {
				return nil, err
			}
			defer proxy.Close()
			return obs, nil
		},
	},
	{
		Group: "provider", Name: "dnspod-record-missing",
		Doc: "a missing DNSPod record is reported the same way",
		Run: func(t *Target) (*Observation, error) {
			obs, proxy, err := dnspodRun(t, `{"status":{"code":"1","message":"ok"},"records":[]}`, nil)
			if err != nil {
				return nil, err
			}
			defer proxy.Close()
			return obs, nil
		},
	},
	{
		Group: "provider", Name: "dnspod-api-error",
		Doc: "a DNSPod API error is reported the same way",
		Run: func(t *Target) (*Observation, error) {
			obs, proxy, err := dnspodRun(t, `{"status":{"code":"6","message":"bad token"}}`, nil)
			if err != nil {
				return nil, err
			}
			defer proxy.Close()
			return obs, nil
		},
	},
	{
		Group: "provider", Name: "dnspod-record-type-mismatch",
		Doc: "a record of the wrong type counts as missing",
		Run: func(t *Target) (*Observation, error) {
			obs, proxy, err := dnspodRun(t, `{"status":{"code":"1","message":"ok"},"records":[{"name":"home","type":"AAAA","id":"12","value":"2001:db8::1","line_id":"0"}]}`, nil)
			if err != nil {
				return nil, err
			}
			defer proxy.Close()
			return obs, nil
		},
	},
	{
		Group: "provider", Name: "dnspod-no-credentials",
		Doc: "an incomplete DNSPod config is rejected before any request",
		Run: func(t *Target) (*Observation, error) {
			t.WriteFile("client.json", clientConf(map[string]any{
				"enable":   map[string]any{"ipv4": true},
				"services": map[string]any{"dnspod": true},
			}))
			t.WriteFile("dnspod.json", dnspodConf(map[string]any{"id": "", "token": ""}))

			res := t.Run("client", "-c", t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "provider", Name: "cloudflare-no-credentials",
		Doc: "an incomplete Cloudflare config is rejected before any request",
		Run: func(t *Target) (*Observation, error) {
			t.WriteFile("client.json", clientConf(map[string]any{
				"enable":   map[string]any{"ipv4": true},
				"services": map[string]any{"cloudflare": true},
			}))
			t.WriteFile("cloudflare.json", cloudflareConf(map[string]any{"zone_id": "", "api_token": ""}))

			res := t.Run("client", "-c", t.Dir)
			return observe(t, res), nil
		},
	},
	{
		Group: "provider", Name: "cloudflare-record-changed",
		Doc: "a changed Cloudflare record is updated with an identical request body",
		Run: func(t *Target) (*Observation, error) {
			return cloudflareRun(t, false)
		},
	},
	{
		Group: "provider", Name: "cloudflare-proxied",
		Doc: "the Cloudflare proxied flag is carried into the update request",
		Run: func(t *Target) (*Observation, error) {
			return cloudflareRun(t, true)
		},
	},
	{
		Group: "provider", Name: "cloudflare-auth-failure",
		Doc: "a Cloudflare authentication failure is reported the same way",
		Run: func(t *Target) (*Observation, error) {
			ipAPI := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
				fmt.Fprint(w, `{"ip":"203.0.113.7"}`)
			})
			defer ipAPI.Close()

			proxy, err := NewProxy(func(host, method, path string, body []byte) Response {
				if strings.Contains(path, "/dns_records") {
					return Response{Status: 200, Body: `{"success":false,"errors":[{"code":1000,"message":"Invalid API Token"}]}`}
				}
				return Response{Status: 404, Body: `{}`}
			})
			if err != nil {
				return nil, err
			}
			defer proxy.Close()

			caFile := filepath.Join(t.Dir, "ca.pem")
			must(proxy.WriteCA(caFile))
			t.ExtraEnv = append(t.ExtraEnv, proxy.Env(caFile)...)

			t.WriteFile("client.json", clientConf(map[string]any{
				"api_url":  map[string]any{"ipv4": ipAPI.URL},
				"enable":   map[string]any{"ipv4": true},
				"services": map[string]any{"cloudflare": true},
			}))
			t.WriteFile("cloudflare.json", cloudflareConf(nil))

			res := t.Run("client", "-c", t.Dir)
			obs := observe(t, res)
			obs.Requests = proxy.Requests()
			return obs, nil
		},
	},
}

func dnspodRun(t *Target, listReply string, modifyReply *string) (*Observation, *Proxy, error) {
	ipAPI := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
		fmt.Fprint(w, `{"ip":"203.0.113.7"}`)
	})
	defer ipAPI.Close()

	proxy, err := NewProxy(func(host, method, path string, body []byte) Response {
		switch {
		case strings.Contains(path, "Record.List"):
			return Response{Status: 200, Body: listReply}
		case strings.Contains(path, "Record.Modify"):
			if modifyReply != nil {
				return Response{Status: 200, Body: *modifyReply}
			}
			return Response{Status: 200, Body: `{"status":{"code":"1","message":"ok"}}`}
		}
		return Response{Status: 404, Body: `{}`}
	})
	if err != nil {
		return nil, nil, err
	}

	caFile := filepath.Join(t.Dir, "ca.pem")
	must(proxy.WriteCA(caFile))
	t.ExtraEnv = append(t.ExtraEnv, proxy.Env(caFile)...)

	t.WriteFile("client.json", clientConf(map[string]any{
		"api_url":  map[string]any{"ipv4": ipAPI.URL},
		"enable":   map[string]any{"ipv4": true},
		"services": map[string]any{"dnspod": true},
	}))
	t.WriteFile("dnspod.json", dnspodConf(nil))

	res := t.Run("client", "-c", t.Dir)
	obs := observe(t, res)
	obs.Requests = proxy.Requests()
	return obs, proxy, nil
}

func cloudflareRun(t *Target, proxied bool) (*Observation, error) {
	ipAPI := NewFakeServer(func(w http.ResponseWriter, r *http.Request, body []byte) {
		fmt.Fprint(w, `{"ip":"203.0.113.7"}`)
	})
	defer ipAPI.Close()

	proxy, err := NewProxy(func(host, method, path string, body []byte) Response {
		if method == http.MethodGet {
			return Response{Status: 200, Body: `{"success":true,"result":[{"id":"rec1","name":"home.example.com","type":"A","content":"198.51.100.1"}]}`}
		}
		return Response{Status: 200, Body: `{"success":true,"result":{}}`}
	})
	if err != nil {
		return nil, err
	}
	defer proxy.Close()

	caFile := filepath.Join(t.Dir, "ca.pem")
	must(proxy.WriteCA(caFile))
	t.ExtraEnv = append(t.ExtraEnv, proxy.Env(caFile)...)

	t.WriteFile("client.json", clientConf(map[string]any{
		"api_url":  map[string]any{"ipv4": ipAPI.URL},
		"enable":   map[string]any{"ipv4": true},
		"services": map[string]any{"cloudflare": true},
	}))
	t.WriteFile("cloudflare.json", cloudflareConf(map[string]any{"proxied": proxied}))

	res := t.Run("client", "-c", t.Dir)
	obs := observe(t, res)
	obs.Requests = proxy.Requests()
	return obs, nil
}

// ---------------------------------------------------------------------------
// scenarios: server
// ---------------------------------------------------------------------------

var serverScenarios = []Scenario{
	{
		Group: "server", Name: "echo-endpoints",
		Doc: "the IP echo endpoint answers the same for the same request headers",
		Run: func(t *Target) (*Observation, error) {
			port, err := freePort()
			if err != nil {
				return nil, err
			}
			t.WriteFile("server.json", serverConf(map[string]any{
				"server_addr": fmt.Sprintf("127.0.0.1:%d", port),
			}))

			obs := &Observation{}
			proc, err := startServer(t, port, obs)
			if err != nil {
				return nil, err
			}
			base := fmt.Sprintf("http://127.0.0.1:%d", port)
			obs.Responses = append(obs.Responses,
				httpDo(http.MethodGet, base+"/", "", nil),
				httpDo(http.MethodGet, base+"/", "", map[string]string{"X-Forwarded-For": "203.0.113.9, 10.0.0.1"}),
				httpDo(http.MethodGet, base+"/", "", map[string]string{"X-Real-IP": "2001:db8::5"}),
			)
			obs.ServerLog = proc.Stop()
			obs.Files = collectFiles(t.Dir)
			return obs, nil
		},
	},
	{
		Group: "server", Name: "echo-wrong-method",
		Doc: "a non-GET request to the echo endpoint is refused",
		Run: func(t *Target) (*Observation, error) {
			port, err := freePort()
			if err != nil {
				return nil, err
			}
			t.WriteFile("server.json", serverConf(map[string]any{
				"server_addr": fmt.Sprintf("127.0.0.1:%d", port),
			}))

			obs := &Observation{}
			proc, err := startServer(t, port, obs)
			if err != nil {
				return nil, err
			}
			obs.Responses = append(obs.Responses, httpDo(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/", port), "{}", nil))
			obs.ServerLog = proc.Stop()
			return obs, nil
		},
	},
	{
		Group: "server", Name: "center-token-flows",
		Doc: "the center endpoint answers the same for each token state",
		Run: func(t *Target) (*Observation, error) {
			port, err := freePort()
			if err != nil {
				return nil, err
			}
			t.WriteFile("server.json", serverConf(map[string]any{
				"server_addr":    fmt.Sprintf("127.0.0.1:%d", port),
				"center_service": true,
			}))
			t.WriteFile("services.json", prettyJSON(map[string]any{
				"dnspod":       map[string]any{"enable": false},
				"alidns":       map[string]any{"enable": false},
				"cloudflare":   map[string]any{"enable": false},
				"huawei_cloud": map[string]any{"enable": false},
			}))
			t.WriteFile("whitelist.json", prettyJSON(map[string]any{
				"0123456789abcdef": map[string]any{
					"enable": true, "description": "home", "service": "dnspod",
					"domain_record": map[string]any{"domain": "example.com", "subdomain": map[string]any{"a": "home", "aaaa": "home"}},
				},
				"fedcba9876543210": map[string]any{
					"enable": false, "description": "off", "service": "dnspod",
					"domain_record": map[string]any{"domain": "example.com", "subdomain": map[string]any{"a": "home", "aaaa": "home"}},
				},
			}))

			obs := &Observation{}
			proc, err := startServer(t, port, obs)
			if err != nil {
				return nil, err
			}
			url := fmt.Sprintf("http://127.0.0.1:%d/center", port)
			jsonHeader := map[string]string{"Content-Type": "application/json"}

			obs.Responses = append(obs.Responses,
				httpDo(http.MethodPost, url, `{"token":"0123456789abcdef","enable":{"ipv4":true},"ip":{"ipv4":"203.0.113.7"}}`, jsonHeader),
				httpDo(http.MethodPost, url, `{"token":"fedcba9876543210","enable":{"ipv4":true},"ip":{"ipv4":"203.0.113.7"}}`, jsonHeader),
				httpDo(http.MethodPost, url, `{"token":"ffffffffffffffff","enable":{"ipv4":true},"ip":{"ipv4":"203.0.113.7"}}`, jsonHeader),
				httpDo(http.MethodPost, url, `{"token":`, jsonHeader),
				httpDo(http.MethodGet, url, "", nil),
			)
			obs.ServerLog = proc.Stop()
			return obs, nil
		},
	},
	{
		Group: "server", Name: "whitelist-cli",
		Doc: "the whitelist CLI writes the same file for insert, update and delete",
		Run: func(t *Target) (*Observation, error) {
			var stdout, stderr strings.Builder
			exit := 0
			for _, args := range [][]string{
				{"-i", "2", "-c", t.Dir},
				{"-a", "-t", "0123456789abcdef", "-s", "dnspod", "-D", "example.com", "-A", "home", "-m", "home", "-c", t.Dir},
				// updating must not erase the entry, so the compared file still
				// contains it (v1 ignored -D here, v2 applies it)
				{"-a", "-t", "0123456789abcdef", "-D", "other.com", "-A", "lab", "-c", t.Dir},
				{"-a", "-t", "fedcba9876543210", "-s", "cloudflare", "-D", "example.com", "-A", "cf", "-c", t.Dir},
				{"-d", "-t", "fedcba9876543210", "-c", t.Dir},
				{"-d", "-t", "fedcba9876543210", "-c", t.Dir},
			} {
				res := t.Run("server", args...)
				exit += res.ExitCode
				stdout.WriteString(res.Stdout)
				stderr.WriteString(res.Stderr)
			}
			obs := &Observation{
				ExitCode: exit,
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				Files:    collectFiles(t.Dir),
			}
			return obs, nil
		},
	},
	{
		Group: "server", Name: "add-validation",
		Doc: "invalid whitelist additions are rejected with the same exit code",
		Run: func(t *Target) (*Observation, error) {
			var stdout, stderr strings.Builder
			exit := 0
			for _, args := range [][]string{
				{"-a", "-t", "short", "-s", "dnspod", "-D", "example.com", "-A", "home", "-c", t.Dir},
				{"-a", "-t", "0123456789abcdef", "-s", "route53", "-D", "example.com", "-A", "home", "-c", t.Dir},
				{"-a", "-t", "0123456789abcdef", "-s", "dnspod", "-A", "home", "-c", t.Dir},
				{"-a", "-t", "0123456789abcdef", "-s", "dnspod", "-D", "example.com", "-c", t.Dir},
				{"-a", "-t", "0123456789abcdef", "-s", "dnspod", "-D", "example.com", "-A", "home", "-m", strings.Repeat("x", 33), "-c", t.Dir},
				{"-d", "-c", t.Dir},
				{"-g", "-l", "5", "-c", t.Dir},
			} {
				res := t.Run("server", args...)
				exit += res.ExitCode
				stdout.WriteString(res.Stdout)
				stderr.WriteString(res.Stderr)
			}
			obs := &Observation{
				ExitCode: exit,
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				Files:    collectFiles(t.Dir),
			}
			return obs, nil
		},
	},
	{
		Group: "server", Name: "token-generation",
		Doc: "generated tokens have the same length and charset",
		Run: func(t *Target) (*Observation, error) {
			res := t.Run("server", "-g", "-l", "48", "-c", t.Dir)
			token := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(res.Stdout), "Token: "))
			obs := &Observation{
				ExitCode: res.ExitCode,
				Stderr:   res.Stderr,
				// the token itself is random: compare its shape instead
				Stdout: fmt.Sprintf("shape len=%d alnum=%v",
					len(token), isAlphaNumeric(token)),
			}
			return obs, nil
		},
	},
	{
		Group: "server", Name: "startup-without-config",
		Doc: "v2 starts from documented defaults; v1 refuses to start",
		Run: func(t *Target) (*Observation, error) {
			port, err := freePort()
			if err != nil {
				return nil, err
			}
			t.ExtraEnv = append(t.ExtraEnv,
				fmt.Sprintf("DDNS_SERVER_ADDR=127.0.0.1:%d", port),
				// otherwise v2 would look the version up on the real root server
				"DDNS_SERVER_IS_ROOT=true",
			)

			obs := &Observation{}
			proc, err := startServer(t, port, obs)
			if err != nil {
				return nil, err
			}
			obs.Responses = append(obs.Responses, httpDo(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/", port), "", nil))
			obs.ServerLog = proc.Stop()
			return obs, nil
		},
	},
	{
		Group: "server", Name: "service-subcommand",
		Doc:     "v2 adds a service subcommand; v1 has none",
		NewInV2: true,
		Why: "v2 feature: the service subcommand (and rejecting an incomplete one) " +
			"does not exist in v1, which treats the words as positional arguments",
		Run: func(t *Target) (*Observation, error) {
			status := t.Run("server", "service", "status", "-c", t.Dir)
			bare := t.Run("server", "service", "-c", t.Dir)
			obs := &Observation{
				ExitCode: status.ExitCode*100 + bare.ExitCode,
				Stdout:   status.Stdout + bare.Stdout,
				Stderr:   status.Stderr + bare.Stderr,
			}
			return obs, nil
		},
	},
}

func isAlphaNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		default:
			return false
		}
	}
	return true
}

// allScenarios is the registry, in report order.
var allScenarios = concat(
	cliScenarios,
	compatScenarios,
	clientScenarios,
	providerScenarios,
	serverScenarios,
)

func concat(groups ...[]Scenario) []Scenario {
	var out []Scenario
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
