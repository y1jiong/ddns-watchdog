package server

import (
	"ddns-watchdog/internal/common"
)

const ServiceConfFilename = "services.json"

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
	if err = common.MarshalAndSave(conf, ConfDir+"/"+ServiceConfFilename); err != nil {
		return
	}

	return "初始化 " + ConfDir + "/" + ServiceConfFilename, nil
}

func (conf *service) LoadConf() (err error) {
	if err = common.LoadAndUnmarshal(ConfDir+"/"+ServiceConfFilename, &conf); err != nil {
		return
	}
	return LoadWhitelist()
}
