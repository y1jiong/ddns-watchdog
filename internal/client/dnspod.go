package client

import (
	"ddns-watchdog/internal/common"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/bitly/go-simplejson"
)

const (
	DNSPodConfFilename = "dnspod.json"
	dnsPodPrefix       = "DNSPod: "
)

type DNSPod struct {
	ID        string           `json:"id"`
	Token     string           `json:"token"`
	Domain    string           `json:"domain"`
	SubDomain common.Subdomain `json:"sub_domain"`
}

func (dpc *DNSPod) InitConf() (msg string, err error) {
	*dpc = DNSPod{
		ID:     "在 https://console.dnspod.cn/account/token/token 获取",
		Domain: "example.com",
		SubDomain: common.Subdomain{
			A:    "A记录子域名",
			AAAA: "AAAA记录子域名",
		},
	}
	dpc.Token = dpc.ID

	return "初始化 " + ConfDir + "/" + DNSPodConfFilename,
		common.MarshalAndSave(dpc, ConfDir+"/"+DNSPodConfFilename)
}

func (dpc *DNSPod) LoadConf() (err error) {
	if err = common.LoadAndUnmarshal(ConfDir+"/"+DNSPodConfFilename, &dpc); err != nil {
		return
	}

	if dpc.ID == "" || dpc.Token == "" || dpc.Domain == "" || (dpc.SubDomain.A == "" && dpc.SubDomain.AAAA == "") {
		return errors.New("请打开配置文件 " + ConfDir + "/" + DNSPodConfFilename + " 检查你的 id, token, domain, sub_domain 并重新启动")
	}
	return
}

func (dpc *DNSPod) Run(enabled common.Enable, ipv4, ipv6 string) (msg []string, errs []error) {
	if ipv4 != "" && enabled.IPv4 && dpc.SubDomain.A != "" {
		// 获取解析记录
		recordId, recordLineId, recordIP, err := dpc.getParseRecord(dpc.SubDomain.A, "A")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv4 {
			// 更新解析记录
			if err = dpc.updateParseRecord(ipv4, recordId, recordLineId, "A", dpc.SubDomain.A); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, dnsPodPrefix+dpc.SubDomain.A+"."+dpc.Domain+" 已更新解析记录 "+ipv4)
			}
		}
	}
	if ipv6 != "" && enabled.IPv6 && dpc.SubDomain.AAAA != "" {
		// 获取解析记录
		recordId, recordLineId, recordIP, err := dpc.getParseRecord(dpc.SubDomain.AAAA, "AAAA")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv6 {
			// 更新解析记录
			if err = dpc.updateParseRecord(ipv6, recordId, recordLineId, "AAAA", dpc.SubDomain.AAAA); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, dnsPodPrefix+dpc.SubDomain.AAAA+"."+dpc.Domain+" 已更新解析记录 "+ipv6)
			}
		}
	}
	return
}

func checkRespondStatus(jsonObj *simplejson.Json) (err error) {
	statusCode := jsonObj.Get("status").Get("code").MustString()
	if statusCode != "1" {
		return errors.New(dnsPodPrefix + statusCode + ": " + jsonObj.Get("status").Get("message").MustString())
	}
	return
}

func (dpc *DNSPod) getParseRecord(subDomain, recordType string) (recordId, recordLineId, recordIP string, err error) {
	postContent := dpc.publicRequestInit()
	postContent = postContent + "&" + dpc.recordRequestInit(subDomain)

	respJson, err := postman("https://dnsapi.cn/Record.List", postContent)
	if err != nil {
		return
	}

	jsonObj, err := simplejson.NewJson(respJson)
	if err != nil {
		return
	}

	if err = checkRespondStatus(jsonObj); err != nil {
		return
	}

	records, err := jsonObj.Get("records").Array()
	if len(records) == 0 {
		err = errors.New(dnsPodPrefix + subDomain + "." + dpc.Domain + " 解析记录不存在")
		return
	}

	for _, value := range records {
		element := value.(map[string]any)
		if element["name"].(string) == subDomain && element["type"].(string) == recordType {
			recordId = element["id"].(string)
			recordIP = element["value"].(string)
			recordLineId = element["line_id"].(string)
			break
		}
	}

	if recordId == "" || recordIP == "" || recordLineId == "" {
		err = errors.New(dnsPodPrefix + subDomain + "." + dpc.Domain + " 的 " + recordType + " 解析记录不存在")
	}
	return
}

func (dpc *DNSPod) updateParseRecord(ipAddr, recordId, recordLineId, recordType, subDomain string) (err error) {
	postContent := dpc.publicRequestInit()
	postContent = postContent + "&" + dpc.recordModifyRequestInit(ipAddr, recordId, recordLineId, recordType, subDomain)

	respJson, err := postman("https://dnsapi.cn/Record.Modify", postContent)
	if err != nil {
		return
	}

	jsonObj, err := simplejson.NewJson(respJson)
	if err != nil {
		return
	}

	return checkRespondStatus(jsonObj)
}

func (dpc *DNSPod) publicRequestInit() (pp string) {
	pp = "login_token=" + dpc.ID + "," + dpc.Token +
		"&format=" + "json" +
		"&lang=" + "cn" +
		"&error_on_empty=" + "no"
	return
}

func (dpc *DNSPod) recordRequestInit(subDomain string) (rr string) {
	rr = "domain=" + dpc.Domain +
		"&sub_domain=" + subDomain
	return
}

func (dpc *DNSPod) recordModifyRequestInit(ipAddr, recordId, recordLineId, recordType, subDomain string) string {
	return "domain=" + dpc.Domain +
		"&record_id=" + recordId +
		"&sub_domain=" + subDomain +
		"&record_type=" + recordType +
		"&record_line_id=" + recordLineId +
		"&value=" + ipAddr
}

func postman(url, src string) (dst []byte, err error) {
	req, err := httpNewRequest(http.MethodPost, url, strings.NewReader(src))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", projName+"/"+common.Version+" ()") // special for DNSPod

	resp, err := common.DefaultHttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
