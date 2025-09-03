package client

import (
	"ddns-watchdog/internal/common"
	"encoding/json"
	"errors"
	"io"
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
	conf.APIUrl.IPv4 = common.DefaultAPIUrl
	conf.APIUrl.IPv6 = common.DefaultIPv6APIUrl
	conf.APIUrl.Version = common.DefaultAPIUrl
	conf.EnableIPv6Fallback = true
	conf.CheckCycleMinutes = 0

	return "初始化 " + ConfDir + "/" + ConfFilename,
		common.MarshalAndSave(conf, ConfDir+"/"+ConfFilename)
}

func (conf *client) LoadConf() (err error) {
	if err = common.LoadAndUnmarshal(ConfDir+"/"+ConfFilename, &conf); err != nil {
		return
	}

	// 检查启用 IP 类型
	if !conf.Enable.IPv4 && !conf.Enable.IPv6 {
		return errors.New("请打开客户端配置文件 " + ConfDir + "/" + ConfFilename + " 启用需要使用的 IP 类型并重新启动")
	}

	// 检查启用服务
	if !conf.Center.Enable &&
		!conf.Services.DNSPod &&
		!conf.Services.AliDNS &&
		!conf.Services.Cloudflare &&
		!conf.Services.HuaweiCloud {
		return errors.New("请打开客户端配置文件 " + ConfDir + "/" + ConfFilename + " 启用需要使用的服务并重新启动")
	}
	return
}

func (conf *client) GetLatestVersion() (str string) {
	resp, err := httpGet(conf.APIUrl.Version)
	if err != nil {
		return "N/A (请检查网络连接)"
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "N/A (数据包错误)"
	}

	var res common.GetIPResp
	if err = json.Unmarshal(body, &res); err != nil {
		return "N/A (数据包错误)"
	}
	if res.Version == "" {
		return "N/A (没有获取到版本信息)"
	}

	return res.Version
}

func (conf *client) CheckLatestVersion() {
	if conf.APIUrl.Version == "" {
		conf.APIUrl.Version = common.DefaultAPIUrl
	}
	common.VersionTips(conf.GetLatestVersion())
}
