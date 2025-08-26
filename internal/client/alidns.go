package client

import (
	"ddns-watchdog/internal/common"
	"errors"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/alidns"
)

const AliDNSConfFileName = "alidns.json"

type AliDNS struct {
	AccessKeyId     string           `json:"access_key_id"`
	AccessKeySecret string           `json:"access_key_secret"`
	Domain          string           `json:"domain"`
	SubDomain       common.Subdomain `json:"sub_domain"`
}

func (ad *AliDNS) InitConf() (msg string, err error) {
	*ad = AliDNS{
		AccessKeyId: "在 https://ram.console.aliyun.com/users 获取",
		Domain:      "example.com",
		SubDomain: common.Subdomain{
			A:    "A记录子域名",
			AAAA: "AAAA记录子域名",
		},
	}
	ad.AccessKeySecret = ad.AccessKeyId

	return "初始化 " + ConfDirectoryName + "/" + AliDNSConfFileName,
		common.MarshalAndSave(ad, ConfDirectoryName+"/"+AliDNSConfFileName)
}

func (ad *AliDNS) LoadConf() (err error) {
	if err = common.LoadAndUnmarshal(ConfDirectoryName+"/"+AliDNSConfFileName, &ad); err != nil {
		return
	}

	if ad.AccessKeyId == "" || ad.AccessKeySecret == "" || ad.Domain == "" || (ad.SubDomain.A == "" && ad.SubDomain.AAAA == "") {
		return errors.New("请打开配置文件 " + ConfDirectoryName + "/" + AliDNSConfFileName + " 检查你的 access_key_id, access_key_secret, domain, sub_domain 并重新启动")
	}
	return
}

func (ad *AliDNS) Run(enabled common.Enable, ipv4, ipv6 string) (msg []string, errs []error) {
	if ipv4 != "" && enabled.IPv4 && ad.SubDomain.A != "" {
		// 获取解析记录
		recordId, recordIP, err := ad.getParseRecord(ad.SubDomain.A, "A")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv4 {
			// 更新解析记录
			if err = ad.updateParseRecord(ipv4, recordId, "A", ad.SubDomain.A); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, "AliDNS: "+ad.SubDomain.A+"."+ad.Domain+" 已更新解析记录 "+ipv4)
			}
		}
	}
	if ipv6 != "" && enabled.IPv6 && ad.SubDomain.AAAA != "" {
		// 获取解析记录
		recordId, recordIP, err := ad.getParseRecord(ad.SubDomain.AAAA, "AAAA")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv6 {
			// 更新解析记录
			if err = ad.updateParseRecord(ipv6, recordId, "AAAA", ad.SubDomain.AAAA); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, "AliDNS: "+ad.SubDomain.AAAA+"."+ad.Domain+" 已更新解析记录 "+ipv6)
			}
		}
	}
	return
}

func (ad *AliDNS) getParseRecord(subDomain, recordType string) (recordId, recordIP string, err error) {
	dnsClient, err := alidns.NewClientWithAccessKey("cn-hangzhou", ad.AccessKeyId, ad.AccessKeySecret)
	if err != nil {
		return
	}

	request := alidns.CreateDescribeDomainRecordsRequest()
	request.Scheme = "https"
	request.DomainName = ad.Domain

	response, err := dnsClient.DescribeDomainRecords(request)
	if err != nil {
		return
	}

	for i := range response.DomainRecords.Record {
		if response.DomainRecords.Record[i].RR == subDomain &&
			response.DomainRecords.Record[i].Type == recordType {
			recordId = response.DomainRecords.Record[i].RecordId
			recordIP = response.DomainRecords.Record[i].Value
			break
		}
	}

	if recordId == "" || recordIP == "" {
		err = errors.New("AliDNS: " + subDomain + "." + ad.Domain + " 的 " + recordType + " 解析记录不存在")
	}
	return
}

func (ad *AliDNS) updateParseRecord(ipAddr, recordId, recordType, subDomain string) (err error) {
	dnsClient, err := alidns.NewClientWithAccessKey("cn-hangzhou", ad.AccessKeyId, ad.AccessKeySecret)
	if err != nil {
		return
	}

	request := alidns.CreateUpdateDomainRecordRequest()
	request.Scheme = "https"
	request.RecordId = recordId
	request.RR = subDomain
	request.Type = recordType
	request.Value = ipAddr

	_, err = dnsClient.UpdateDomainRecord(request)
	return
}
