package client

import (
	"ddns-watchdog/internal/common"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
)

const ConfFilename = "client.json"

type client struct {
	APIUrl             apiUrl        `json:"api_url"`
	Center             center        `json:"center"`
	Enable             common.Enable `json:"enable"`
	NetworkCard        networkCard   `json:"network_card"`
	Services           service       `json:"services"`
	EnableIPv6Fallback bool          `json:"enable_ipv6_fallback"`
	CheckCycleMinutes  int           `json:"check_cycle_minutes"`
	LatestIPv4         string        `json:"-"`
	LatestIPv6         string        `json:"-"`
}

type apiUrl struct {
	IPv4    string `json:"ipv4"`
	IPv6    string `json:"ipv6"`
	Version string `json:"version"`
}

type center struct {
	APIUrl string `json:"api_url"`
	Token  string `json:"token"`
	Enable bool   `json:"enable"`
}

type networkCard struct {
	Enable bool   `json:"enable"`
	IPv4   string `json:"ipv4"`
	IPv6   string `json:"ipv6"`
}

type service struct {
	DNSPod      bool `json:"dnspod"`
	AliDNS      bool `json:"alidns"`
	Cloudflare  bool `json:"cloudflare"`
	HuaweiCloud bool `json:"huawei_cloud"`
}

func (conf *client) InitConf() (msg string, err error) {
	*conf = client{}
	conf.applyDefaults()

	return "initialized " + ConfDir + "/" + ConfFilename,
		common.MarshalAndSave(conf, ConfDir+"/"+ConfFilename)
}

// applyDefaults seeds the values InitConf() writes into a new config file.
// LoadConf runs it before unmarshalling, so a missing or partial file (for
// example a Docker deployment configured from the environment) behaves like the
// generated template. Notably, enable_ipv6_fallback is documented as defaulting
// to true.
func (conf *client) applyDefaults() {
	conf.APIUrl.IPv4 = common.DefaultAPIUrl
	conf.APIUrl.IPv6 = common.DefaultIPv6APIUrl
	conf.APIUrl.Version = common.DefaultAPIUrl
	conf.EnableIPv6Fallback = true
}

func (conf *client) LoadConf() (err error) {
	conf.applyDefaults()

	// Missing file is allowed: Docker users configure via env vars only.
	missing := false
	if err = common.LoadAndUnmarshal(ConfDir+"/"+ConfFilename, &conf); err != nil {
		if !os.IsNotExist(err) {
			return
		}
		missing = true
	}
	err = nil

	conf.applyEnvOverrides()

	if !conf.Enable.IPv4 && !conf.Enable.IPv6 {
		return errors.New(confHint(`no IP type enabled: set "enable"."ipv4" or "enable"."ipv6" to true`, missing))
	}

	if !conf.Center.Enable &&
		!conf.Services.DNSPod &&
		!conf.Services.AliDNS &&
		!conf.Services.Cloudflare &&
		!conf.Services.HuaweiCloud {
		return errors.New(confHint(`no service enabled: enable a provider in "services" or set "center"."enable" to true`, missing))
	}
	return
}

// confHint says where to fix a config problem, and how to create the file when it
// does not exist yet - the state a fresh "nothing configured" run ends up in.
func confHint(msg string, missing bool) string {
	hint := "; edit " + ConfDir + "/" + ConfFilename
	if missing {
		hint += " (run with -i 0 to create it)"
	}
	return msg + hint
}

func (conf *client) applyEnvOverrides() {
	if v := os.Getenv("DDNS_API_URL_IPV4"); v != "" {
		conf.APIUrl.IPv4 = v
	}
	if v := os.Getenv("DDNS_API_URL_IPV6"); v != "" {
		conf.APIUrl.IPv6 = v
	}
	if v := os.Getenv("DDNS_API_URL_VERSION"); v != "" {
		conf.APIUrl.Version = v
	}
	if v := os.Getenv("DDNS_CENTER_ENABLE"); v != "" {
		conf.Center.Enable = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_CENTER_URL"); v != "" {
		conf.Center.APIUrl = v
	}
	if v := os.Getenv("DDNS_CENTER_TOKEN"); v != "" {
		conf.Center.Token = v
	}
	if v := os.Getenv("DDNS_ENABLE_IPV4"); v != "" {
		conf.Enable.IPv4 = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_ENABLE_IPV6"); v != "" {
		conf.Enable.IPv6 = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_NETWORK_CARD_ENABLE"); v != "" {
		conf.NetworkCard.Enable = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_NETWORK_CARD_IPV4"); v != "" {
		conf.NetworkCard.IPv4 = v
	}
	if v := os.Getenv("DDNS_NETWORK_CARD_IPV6"); v != "" {
		conf.NetworkCard.IPv6 = v
	}
	if v := os.Getenv("DDNS_SERVICE_DNSPOD"); v != "" {
		conf.Services.DNSPod = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_SERVICE_ALIDNS"); v != "" {
		conf.Services.AliDNS = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_SERVICE_CLOUDFLARE"); v != "" {
		conf.Services.Cloudflare = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_SERVICE_HUAWEI"); v != "" {
		conf.Services.HuaweiCloud = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_IPV6_FALLBACK"); v != "" {
		conf.EnableIPv6Fallback = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_CHECK_CYCLE"); v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			conf.CheckCycleMinutes = n
		}
	}
}

func (conf *client) GetLatestVersion() (str string) {
	resp, err := httpGet(conf.APIUrl.Version)
	if err != nil {
		return "N/A (check network connection)"
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "N/A (invalid response)"
	}

	var res common.GetIPResp
	if err = json.Unmarshal(body, &res); err != nil {
		return "N/A (invalid response)"
	}
	if res.Version == "" {
		return "N/A (version info not found)"
	}

	return res.Version
}

func (conf *client) CheckLatestVersion() {
	if conf.APIUrl.Version == "" {
		conf.APIUrl.Version = common.DefaultAPIUrl
	}
	common.VersionTips(conf.GetLatestVersion())
}
