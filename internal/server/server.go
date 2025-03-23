package server

import (
	"ddns-watchdog/internal/common"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	ConfFileName = "server.json"
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
		ServerAddr:    ":10032",
		IsRootServer:  false,
		RootServerUrl: "https://yzyweb.cn/ddns-watchdog",
		Route: route{
			GetIP:  "/",
			Center: "/center",
		},
	}
	if err = common.MarshalAndSave(conf, ConfDirectoryName+"/"+ConfFileName); err != nil {
		return
	}
	return "初始化 " + ConfDirectoryName + "/" + ConfFileName, nil
}

func (conf *server) LoadConf() (err error) {
	return common.LoadAndUnmarshal(ConfDirectoryName+"/"+ConfFileName, &conf)
}

func (conf *server) GetLatestVersion() (str string) {
	if !conf.IsRootServer {
		resp, err := http.Get(conf.RootServerUrl)
		if err != nil {
			return "N/A (请检查网络连接)"
		}
		defer func(Body io.ReadCloser) {
			if err = Body.Close(); err != nil {
				str = err.Error()
			}
		}(resp.Body)
		recvJson, err := io.ReadAll(resp.Body)
		if err != nil {
			return "N/A (数据包错误)"
		}
		var recv = common.GetIPResp{}
		if err = json.Unmarshal(recvJson, &recv); err != nil {
			return "N/A (数据包错误)"
		}
		if recv.Version == "" {
			return "N/A (没有获取到版本信息)"
		}
		return recv.Version
	}
	return common.LocalVersion
}

func (conf *server) CheckLatestVersion() {
	if !conf.IsRootServer {
		common.VersionTips(conf.GetLatestVersion())
	} else {
		fmt.Println("本机是根服务器")
		fmt.Println("当前版本", common.LocalVersion)
		fmt.Println("Git Commit:", common.GitCommit)
		fmt.Println("Build Time:", common.BuildTime)
	}
}
