package client

import (
	"ddns-watchdog/internal/common"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/bitly/go-simplejson"
)

const CloudflareConfFileName = "cloudflare.json"

type Cloudflare struct {
	ZoneID   string           `json:"zone_id"`
	APIToken string           `json:"api_token"`
	Domain   common.Subdomain `json:"domain"`
	Proxied  bool             `json:"proxied"`
}

type cloudflareUpdateRequest struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	Ttl     int    `json:"ttl"`
}

func (cfc *Cloudflare) InitConf() (msg string, err error) {
	*cfc = Cloudflare{
		ZoneID:   "在你域名页面的右下角有个区域 ID",
		APIToken: "在 https://dash.cloudflare.com/profile/api-tokens 获取",
		Domain: common.Subdomain{
			A:    "A记录子域名.example.com",
			AAAA: "AAAA记录子域名.example.com",
		},
	}

	return "初始化 " + ConfDirectoryName + "/" + CloudflareConfFileName,
		common.MarshalAndSave(cfc, ConfDirectoryName+"/"+CloudflareConfFileName)
}

func (cfc *Cloudflare) LoadConf() (err error) {
	if err = common.LoadAndUnmarshal(ConfDirectoryName+"/"+CloudflareConfFileName, &cfc); err != nil {
		return
	}

	if cfc.ZoneID == "" || cfc.APIToken == "" || (cfc.Domain.A == "" && cfc.Domain.AAAA == "") {
		return errors.New("请打开配置文件 " + ConfDirectoryName + "/" + CloudflareConfFileName + " 检查你的 zone_id, api_token, domain 并重新启动")
	}
	return
}

func (cfc *Cloudflare) Run(enabled common.Enable, ipv4, ipv6 string) (msg []string, errs []error) {
	if ipv4 != "" && enabled.IPv4 && cfc.Domain.A != "" {
		// 获取解析记录
		domainId, recordIP, err := cfc.getParseRecord(cfc.Domain.A, "A")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv4 {
			// 更新解析记录
			if err = cfc.updateParseRecord(ipv4, domainId, "A", cfc.Domain.A); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, "Cloudflare: "+cfc.Domain.A+" 已更新解析记录 "+ipv4)
			}
		}
	}
	if ipv6 != "" && enabled.IPv6 && cfc.Domain.AAAA != "" {
		// 获取解析记录
		domainId, recordIP, err := cfc.getParseRecord(cfc.Domain.AAAA, "AAAA")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv6 {
			// 更新解析记录
			if err = cfc.updateParseRecord(ipv6, domainId, "AAAA", cfc.Domain.AAAA); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, "Cloudflare: "+cfc.Domain.AAAA+" 已更新解析记录 "+ipv6)
			}
		}
	}
	return
}

func (cfc *Cloudflare) getParseRecord(domain, recordType string) (domainId, recordIP string, err error) {
	url := "https://api.cloudflare.com/client/v4/zones/" + cfc.ZoneID + "/dns_records?name=" + domain
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return
	}

	req.Header.Set("Authorization", "Bearer "+cfc.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := getGeneralHttpClient().Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respJson, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	jsonObj, err := simplejson.NewJson(respJson)
	if err != nil {
		return
	}

	if errMsg := jsonObj.Get("error").MustString(); errMsg != "" {
		err = errors.New("Cloudflare: " + errMsg)
		return
	}
	if !jsonObj.Get("success").MustBool() {
		err = errors.New("Cloudflare: 身份认证似乎有问题")
		return
	}

	records, err := jsonObj.Get("result").Array()
	if len(records) == 0 {
		err = errors.New("Cloudflare: " + domain + " 解析记录不存在")
		return
	}

	for _, v := range records {
		element := v.(map[string]any)
		if element["name"].(string) == domain && element["type"].(string) == recordType {
			domainId = element["id"].(string)
			recordIP = element["content"].(string)
			break
		}
	}

	if domainId == "" || recordIP == "" {
		err = errors.New("Cloudflare: " + domain + " 的 " + recordType + " 解析记录不存在")
	}
	return
}

func (cfc *Cloudflare) updateParseRecord(ipAddr, domainId, recordType, domain string) (err error) {
	url := "https://api.cloudflare.com/client/v4/zones/" + cfc.ZoneID + "/dns_records/" + domainId
	reqData := cloudflareUpdateRequest{
		Type:    recordType,
		Name:    domain,
		Content: ipAddr,
		Proxied: cfc.Proxied,
		Ttl:     1,
	}

	reqJson, err := json.Marshal(reqData)
	if err != nil {
		return
	}

	req, err := http.NewRequest("PUT", url, strings.NewReader(string(reqJson)))
	if err != nil {
		return
	}

	req.Header.Set("Authorization", "Bearer "+cfc.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := getGeneralHttpClient().Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respJson, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	jsonObj, err := simplejson.NewJson(respJson)
	if err != nil {
		return
	}

	if getErr := jsonObj.Get("error").MustString(); getErr != "" {
		err = errors.New(getErr)
		return
	}

	if !jsonObj.Get("success").MustBool() {
		var errorsMsg string
		errorsArr, getErr := jsonObj.Get("errors").Array()
		if getErr != nil {
			err = getErr
			return
		}

		for _, value := range errorsArr {
			element := value.(map[string]any)
			errCode := element["code"].(json.Number)
			errMsg := element["message"].(string)
			errorsMsg = errorsMsg + "Cloudflare: " + errCode.String() + ": " + errMsg + "\n"
		}

		err = errors.New(errorsMsg)
	}
	return
}
