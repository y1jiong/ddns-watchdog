package server

import (
	"ddns-watchdog/internal/common"
	"encoding/json"
	"fmt"
	"io"
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

	return "初始化 " + ConfDir + "/" + ConfFilename, nil
}

func (conf *server) LoadConf() (err error) {
	return common.LoadAndUnmarshal(ConfDir+"/"+ConfFilename, &conf)
}

func (conf *server) GetLatestVersion() (str string) {
	if !conf.IsRootServer {
		resp, err := httpGet(conf.RootServerUrl)
		if err != nil {
			return "N/A (请检查网络连接)"
		}
		defer resp.Body.Close()

		respJson, err := io.ReadAll(resp.Body)
		if err != nil {
			return "N/A (数据包错误)"
		}

		var res common.GetIPResp
		if err = json.Unmarshal(respJson, &res); err != nil {
			return "N/A (数据包错误)"
		}

		if res.Version == "" {
			return "N/A (没有获取到版本信息)"
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

	fmt.Println("本机是根服务器")
	fmt.Println("当前版本", common.Version)
	fmt.Println("Git Commit:", common.GitCommit)
	fmt.Println("Build Time:", common.BuildTime)
}
