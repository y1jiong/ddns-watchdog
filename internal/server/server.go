package server

import (
	"cmp"
	"ddns-watchdog/internal/common"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"
)

const (
	ConfFilename = "server.json"

	defaultServerAddr  = ":10032"
	defaultRouteGetIP  = "/"
	defaultRouteCenter = "/center"
)

type server struct {
	ServerAddr    string `json:"server_addr"`
	IsRootServer  bool   `json:"is_root_server"`
	RootServerUrl string `json:"root_server_url"`
	CenterService bool   `json:"center_service"`
	Route         route  `json:"route"`
	TLS           tls    `json:"tls"`
}

type tls struct {
	Enable   bool   `json:"enable"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

type route struct {
	GetIP  string `json:"get_ip"`
	Center string `json:"center"`
}

func (conf *server) InitConf() (msg string, err error) {
	*conf = server{
		ServerAddr:    defaultServerAddr,
		RootServerUrl: common.DefaultAPIUrl,
		Route: route{
			GetIP:  defaultRouteGetIP,
			Center: defaultRouteCenter,
		},
	}
	if err = common.MarshalAndSave(conf, ConfDir+"/"+ConfFilename); err != nil {
		return
	}

	return "initialized " + ConfDir + "/" + ConfFilename, nil
}

// LoadVersionConf loads only what -V needs: whether this is the root server and
// which upstream to ask. A missing server.json is not an error - the defaults
// apply - and route/TLS validation does not matter for a version check.
func (conf *server) LoadVersionConf() (err error) {
	if err = common.LoadAndUnmarshal(ConfDir+"/"+ConfFilename, &conf); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return
	}
	err = nil

	conf.applyEnvOverrides()
	conf.applyDefaults()
	return
}

func (conf *server) LoadConf() (err error) {
	// Missing file is allowed: Docker users configure via env vars only.
	if err = common.LoadAndUnmarshal(ConfDir+"/"+ConfFilename, &conf); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return
	}
	err = nil

	conf.applyEnvOverrides()
	conf.applyDefaults()
	return conf.validate()
}

// applyDefaults fills in what InitConf() would have written. Without it an env-only
// (Docker) deployment keeps ServerAddr and the routes empty, and net/http then binds
// :80 while http.HandleFunc("") panics on the empty route pattern.
func (conf *server) applyDefaults() {
	conf.ServerAddr = cmp.Or(conf.ServerAddr, defaultServerAddr)
	conf.RootServerUrl = cmp.Or(conf.RootServerUrl, common.DefaultAPIUrl)
	conf.Route.GetIP = cmp.Or(conf.Route.GetIP, defaultRouteGetIP)
	conf.Route.Center = cmp.Or(conf.Route.Center, defaultRouteCenter)
}

// validate rejects configs that would otherwise fail later with an obscure runtime error.
func (conf *server) validate() error {
	if conf.Route.GetIP == "" {
		return errors.New("route.get_ip is empty, edit " + ConfDir + "/" + ConfFilename)
	}
	if conf.CenterService {
		if conf.Route.Center == "" {
			return errors.New("route.center is empty, edit " + ConfDir + "/" + ConfFilename)
		}
		if conf.Route.Center == conf.Route.GetIP {
			return errors.New("route.center and route.get_ip must differ, edit " + ConfDir + "/" + ConfFilename)
		}
	}
	if conf.TLS.Enable && (conf.TLS.CertFile == "" || conf.TLS.KeyFile == "") {
		return errors.New("tls is enabled but cert_file or key_file is empty, edit " + ConfDir + "/" + ConfFilename)
	}
	return nil
}

func (conf *server) applyEnvOverrides() {
	if v := os.Getenv("DDNS_SERVER_ADDR"); v != "" {
		conf.ServerAddr = v
	}
	if v := os.Getenv("DDNS_SERVER_IS_ROOT"); v != "" {
		conf.IsRootServer = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_SERVER_ROOT_URL"); v != "" {
		conf.RootServerUrl = v
	}
	if v := os.Getenv("DDNS_SERVER_CENTER"); v != "" {
		conf.CenterService = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_SERVER_ROUTE_GETIP"); v != "" {
		conf.Route.GetIP = v
	}
	if v := os.Getenv("DDNS_SERVER_ROUTE_CENTER"); v != "" {
		conf.Route.Center = v
	}
	if v := os.Getenv("DDNS_SERVER_TLS"); v != "" {
		conf.TLS.Enable = v == "true" || v == "1"
	}
	if v := os.Getenv("DDNS_SERVER_TLS_CERT"); v != "" {
		conf.TLS.CertFile = v
	}
	if v := os.Getenv("DDNS_SERVER_TLS_KEY"); v != "" {
		conf.TLS.KeyFile = v
	}
}

// versionCacheTTL bounds how often the root server is asked for its version.
const versionCacheTTL = 10 * time.Minute

// The advertised version is cached in memory: the IP echo endpoint is
// unauthenticated, so looking the version up on every request would let anyone
// amplify traffic against the configured root server. Failures are cached too, so
// an unreachable root server is not retried on every request either.
var (
	versionCacheMu  sync.Mutex
	versionCache    string
	versionCacheErr error
	versionCached   time.Time
)

func (conf *server) latestVersion() (string, error) {
	if conf.IsRootServer {
		return common.Version, nil
	}

	versionCacheMu.Lock()
	defer versionCacheMu.Unlock()

	if time.Since(versionCached) < versionCacheTTL {
		return versionCache, versionCacheErr
	}

	versionCache, versionCacheErr = conf.fetchLatestVersion()
	versionCached = time.Now()
	return versionCache, versionCacheErr
}

func (conf *server) GetLatestVersion() (str string) {
	version, err := conf.latestVersion()
	if err != nil {
		// The echo endpoint is unauthenticated; do not leak the upstream address
		// or the exact failure to every caller.
		return "N/A (upstream unavailable)"
	}
	return version
}

// fetchLatestVersion asks the configured root server which version it serves.
func (conf *server) fetchLatestVersion() (string, error) {
	return common.LatestVersion(conf.RootServerUrl, projName+"/"+common.Version)
}

func (conf *server) CheckLatestVersion() {
	if conf.IsRootServer {
		fmt.Println("this is the root server")
		common.VersionTips("", nil)
		return
	}

	common.VersionTips(conf.RootServerUrl, conf.latestVersion)
}
