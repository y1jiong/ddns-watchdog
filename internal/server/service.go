package server

import (
	"ddns-watchdog/internal/common"
)

const ServiceConfFileName = "services.json"

type service struct {
	DNSPod      dnspod      `json:"dnspod"`
	AliDNS      alidns      `json:"alidns"`
	Cloudflare  cloudflare  `json:"cloudflare"`
	HuaweiCloud huaweiCloud `json:"huawei_cloud"`
}

type dnspod struct {
	Enable bool   `json:"enable"`
	ID     string `json:"id"`
	Token  string `json:"token"`
}

type alidns struct {
	Enable          bool   `json:"enable"`
	AccessKeyId     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
}

type cloudflare struct {
	Enable   bool   `json:"enable"`
	ZoneID   string `json:"zone_id"`
	APIToken string `json:"api_token"`
}

type huaweiCloud struct {
	Enable          bool   `json:"enable"`
	AccessKeyId     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

func (conf *service) InitConf() (msg string, err error) {
	*conf = service{}
	if err = common.MarshalAndSave(conf, ConfDirectoryName+"/"+ServiceConfFileName); err != nil {
		return
	}

	return "初始化 " + ConfDirectoryName + "/" + ServiceConfFileName, nil
}

func (conf *service) LoadConf() (err error) {
	if err = common.LoadAndUnmarshal(ConfDirectoryName+"/"+ServiceConfFileName, &conf); err != nil {
		return
	}
	return LoadWhitelist()
}
