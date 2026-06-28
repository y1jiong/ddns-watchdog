package server

import (
	"ddns-watchdog/internal/common"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const ConfFilename = "server.json"

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
		ServerAddr:    ":10032",
		RootServerUrl: common.DefaultAPIUrl,
		Route: route{
			GetIP:  "/",
			Center: "/center",
		},
	}
	if err = common.MarshalAndSave(conf, ConfDir+"/"+ConfFilename); err != nil {
		return
	}

	return "initialized " + ConfDir + "/" + ConfFilename, nil
}

func (conf *server) LoadConf() (err error) {
	if err = common.LoadAndUnmarshal(ConfDir+"/"+ConfFilename, &conf); err != nil && !os.IsNotExist(err) {
		return
	}
	err = nil
	conf.applyEnvOverrides()
	return
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

func (conf *server) GetLatestVersion() (str string) {
	if !conf.IsRootServer {
		resp, err := httpGet(conf.RootServerUrl)
		if err != nil {
			return "N/A (check network connection)"
		}
		defer resp.Body.Close()

		respJson, err := io.ReadAll(resp.Body)
		if err != nil {
			return "N/A (invalid response)"
		}

		var res common.GetIPResp
		if err = json.Unmarshal(respJson, &res); err != nil {
			return "N/A (invalid response)"
		}

		if res.Version == "" {
			return "N/A (version info not found)"
		}

		return res.Version
	}
	return common.Version
}

func (conf *server) CheckLatestVersion() {
	if !conf.IsRootServer {
		common.VersionTips(conf.GetLatestVersion())
		return
	}

	fmt.Println("this is the root server")
	fmt.Println("current version:", common.Version)
	fmt.Println("git commit:", common.GitCommit)
	fmt.Println("build time:", common.BuildTime)
}
