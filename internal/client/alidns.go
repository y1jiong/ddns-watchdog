package client

import (
	"ddns-watchdog/internal/common"
	"errors"
	"os"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/alidns"
)

const (
	AliDNSConfFilename = "alidns.json"
	aliDNSPrefix       = "AliDNS: "
)

type AliDNS struct {
	AccessKeyId     string           `json:"access_key_id"`
	AccessKeySecret string           `json:"access_key_secret"`
	Domain          string           `json:"domain"`
	SubDomain       common.Subdomain `json:"sub_domain"`
}

func (ad *AliDNS) InitConf() (msg string, err error) {
	*ad = AliDNS{
		AccessKeyId: "get from https://ram.console.aliyun.com/users",
		Domain:      "example.com",
		SubDomain: common.Subdomain{
			A:    "subdomain for A record",
			AAAA: "subdomain for AAAA record",
		},
	}
	ad.AccessKeySecret = ad.AccessKeyId

	return "initialized " + ConfDir + "/" + AliDNSConfFilename,
		common.MarshalAndSave(ad, ConfDir+"/"+AliDNSConfFilename)
}

func (ad *AliDNS) LoadConf() (err error) {
	if err = common.LoadAndUnmarshal(ConfDir+"/"+AliDNSConfFilename, &ad); err != nil && !os.IsNotExist(err) {
		return
	}
	err = nil

	if v := os.Getenv("DDNS_ALIDNS_AK_ID"); v != "" {
		ad.AccessKeyId = v
	}
	if v := os.Getenv("DDNS_ALIDNS_AK_SECRET"); v != "" {
		ad.AccessKeySecret = v
	}
	if v := os.Getenv("DDNS_ALIDNS_DOMAIN"); v != "" {
		ad.Domain = v
	}
	if v := os.Getenv("DDNS_ALIDNS_SUB_A"); v != "" {
		ad.SubDomain.A = v
	}
	if v := os.Getenv("DDNS_ALIDNS_SUB_AAAA"); v != "" {
		ad.SubDomain.AAAA = v
	}

	if ad.AccessKeyId == "" || ad.AccessKeySecret == "" || ad.Domain == "" || (ad.SubDomain.A == "" && ad.SubDomain.AAAA == "") {
		return errors.New("check access_key_id, access_key_secret, domain, sub_domain in " + ConfDir + "/" + AliDNSConfFilename)
	}
	return
}

func (ad *AliDNS) Run(enabled common.Enable, ipv4, ipv6 string) (msg []string, errs []error) {
	if ipv4 != "" && enabled.IPv4 && ad.SubDomain.A != "" {
		recordId, recordIP, err := ad.getParseRecord(ad.SubDomain.A, "A")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv4 {
			if err = ad.updateParseRecord(ipv4, recordId, "A", ad.SubDomain.A); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, aliDNSPrefix+ad.SubDomain.A+"."+ad.Domain+" record updated to "+ipv4)
			}
		}
	}
	if ipv6 != "" && enabled.IPv6 && ad.SubDomain.AAAA != "" {
		recordId, recordIP, err := ad.getParseRecord(ad.SubDomain.AAAA, "AAAA")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv6 {
			if err = ad.updateParseRecord(ipv6, recordId, "AAAA", ad.SubDomain.AAAA); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, aliDNSPrefix+ad.SubDomain.AAAA+"."+ad.Domain+" record updated to "+ipv6)
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
		err = errors.New(aliDNSPrefix + subDomain + "." + ad.Domain + " " + recordType + " record not found")
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
