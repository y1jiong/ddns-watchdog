package client

import (
	"ddns-watchdog/internal/common"
	"errors"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	dns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"
)

const HuaweiCloudConfFilename = "huaweicloud.json"

type HuaweiCloud struct {
	AccessKeyId     string           `json:"access_key_id"`
	SecretAccessKey string           `json:"secret_access_key"`
	ZoneName        string           `json:"zone_name"`
	Domain          common.Subdomain `json:"domain"`
	ZoneId          string           `json:"-"`
}

func (hc *HuaweiCloud) InitConf() (msg string, err error) {
	*hc = HuaweiCloud{
		AccessKeyId: "在 https://console.huaweicloud.com/iam/ 获取",
		ZoneName:    "example.com.",
		Domain: common.Subdomain{
			A:    "A记录子域名.example.com.",
			AAAA: "AAAA记录子域名.example.com.",
		},
	}
	hc.SecretAccessKey = hc.AccessKeyId

	return "初始化 " + ConfDir + "/" + HuaweiCloudConfFilename,
		common.MarshalAndSave(hc, ConfDir+"/"+HuaweiCloudConfFilename)
}

func (hc *HuaweiCloud) LoadConf() (err error) {
	if err = common.LoadAndUnmarshal(ConfDir+"/"+HuaweiCloudConfFilename, &hc); err != nil {
		return
	}
	if hc.AccessKeyId == "" || hc.SecretAccessKey == "" || hc.ZoneName == "" || (hc.Domain.A == "" && hc.Domain.AAAA == "") {
		return errors.New("请打开配置文件 " + ConfDir + "/" + HuaweiCloudConfFilename + " 检查你的 access_key_id, secret_access_key, domain 并重新启动")
	}
	return
}

func (hc *HuaweiCloud) Run(enabled common.Enable, ipv4, ipv6 string) (msg []string, errs []error) {
	if hc.ZoneId == "" && (enabled.IPv4 || enabled.IPv6) {
		if err := hc.getZoneId(); err != nil {
			errs = append(errs, err)
			return
		}
	}
	if ipv4 != "" && enabled.IPv4 && hc.Domain.A != "" {
		recordSetId, recordIP, err := hc.getParseRecord(hc.Domain.A, "A")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv4 {
			if err = hc.updateParseRecord(ipv4, recordSetId, "A", hc.Domain.A); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, "HuaweiCloud: "+hc.Domain.A+" 已更新解析记录 "+ipv4)
			}
		}
	}
	if ipv6 != "" && enabled.IPv6 && hc.Domain.AAAA != "" {
		recordSetId, recordIP, err := hc.getParseRecord(hc.Domain.AAAA, "AAAA")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv6 {
			if err = hc.updateParseRecord(ipv6, recordSetId, "AAAA", hc.Domain.AAAA); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, "HuaweiCloud: "+hc.Domain.AAAA+" 已更新解析记录 "+ipv6)
			}
		}
	}
	return
}

func (hc *HuaweiCloud) getZoneId() (err error) {
	auth, err := basic.NewCredentialsBuilder().
		WithAk(hc.AccessKeyId).
		WithSk(hc.SecretAccessKey).
		SafeBuild()
	if err != nil {
		return
	}

	hr, err := region.SafeValueOf("cn-east-3")
	if err != nil {
		return
	}

	hhc, err := dns.DnsClientBuilder().
		WithRegion(hr).
		WithCredential(auth).
		SafeBuild()
	if err != nil {
		return
	}

	request := &model.ListPublicZonesRequest{}
	response, err := dns.NewDnsClient(hhc).ListPublicZones(request)
	if err != nil {
		return
	}
	for _, v := range *response.Zones {
		if *v.Name == hc.ZoneName {
			hc.ZoneId = *v.Id
		}
	}
	if hc.ZoneId == "" {
		err = errors.New("HuaweiCloud: " + hc.ZoneName + " Zone 不存在")
	}
	return
}

func (hc *HuaweiCloud) getParseRecord(domain, recordType string) (recordSetId, recordIP string, err error) {
	auth, err := basic.NewCredentialsBuilder().
		WithAk(hc.AccessKeyId).
		WithSk(hc.SecretAccessKey).
		SafeBuild()
	if err != nil {
		return
	}

	hr, err := region.SafeValueOf("cn-east-3")
	if err != nil {
		return
	}

	hhc, err := dns.DnsClientBuilder().
		WithRegion(hr).
		WithCredential(auth).
		SafeBuild()
	if err != nil {
		return
	}

	request := &model.ListRecordSetsByZoneRequest{}
	request.ZoneId = hc.ZoneId

	response, err := dns.NewDnsClient(hhc).ListRecordSetsByZone(request)
	if err != nil {
		return
	}

	for _, v := range *response.Recordsets {
		if *v.Name == domain && *v.Type == recordType {
			recordSetId = *v.Id
			for _, vv := range *v.Records {
				recordIP = vv
			}
			break
		}
	}

	if recordSetId == "" || recordIP == "" {
		err = errors.New("HuaweiCloud: " + domain + " 的 " + recordType + " 解析记录不存在")
	}
	return
}

func (hc *HuaweiCloud) updateParseRecord(ipAddr, recordSetId, recordType, domain string) (err error) {
	auth, err := basic.NewCredentialsBuilder().
		WithAk(hc.AccessKeyId).
		WithSk(hc.SecretAccessKey).
		SafeBuild()
	if err != nil {
		return
	}

	hr, err := region.SafeValueOf("cn-east-3")
	if err != nil {
		return
	}

	hhc, err := dns.DnsClientBuilder().
		WithRegion(hr).
		WithCredential(auth).
		SafeBuild()
	if err != nil {
		return
	}

	request := &model.UpdateRecordSetRequest{}
	request.ZoneId = hc.ZoneId
	request.RecordsetId = recordSetId
	listRecordsBody := []string{ipAddr}
	request.Body = &model.UpdateRecordSetReq{
		Records: &listRecordsBody,
		Type:    &recordType,
		Name:    &domain,
	}

	_, err = dns.NewDnsClient(hhc).UpdateRecordSet(request)
	return
}
