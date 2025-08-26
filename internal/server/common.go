package server

import (
	"ddns-watchdog/internal/client"
	"ddns-watchdog/internal/common"
	"net/http"
)

const (
	InsertSign = "INSERT"
	UpdateSign = "UPDATE"
	DeleteSign = "DELETE"
)

var (
	whitelist map[string]whitelistStruct
)

type domainRecord struct {
	Domain    string           `json:"domain"`
	Subdomain common.Subdomain `json:"subdomain"`
}

type whitelistStruct struct {
	Enable       bool         `json:"enable"`
	Description  string       `json:"description"`
	Service      string       `json:"service"`
	DomainRecord domainRecord `json:"domain_record"`
}

func doVirtualClient(body common.CenterReq, instance whitelistStruct) (httpStatus int, respBody common.GeneralResp, err error) {
	httpStatus = http.StatusOK
	var (
		msg  []string
		errs []error
	)

	switch instance.Service {
	case common.DNSPod:
		if !Services.DNSPod.Enable {
			httpStatus = http.StatusForbidden
			return
		}

		// 初始化虚拟客户端
		dp := client.DNSPod{
			ID:        Services.DNSPod.ID,
			Token:     Services.DNSPod.Token,
			Domain:    instance.DomainRecord.Domain,
			SubDomain: instance.DomainRecord.Subdomain,
		}

		msg, errs = dp.Run(body.Enable, body.IP.IPv4, body.IP.IPv6)
	case common.AliDNS:
		if !Services.AliDNS.Enable {
			httpStatus = http.StatusForbidden
			return
		}

		// 初始化虚拟客户端
		ad := client.AliDNS{
			AccessKeyId:     Services.AliDNS.AccessKeyId,
			AccessKeySecret: Services.AliDNS.AccessKeySecret,
			Domain:          instance.DomainRecord.Domain,
			SubDomain:       instance.DomainRecord.Subdomain,
		}

		msg, errs = ad.Run(body.Enable, body.IP.IPv4, body.IP.IPv6)
	case common.Cloudflare:
		if !Services.Cloudflare.Enable {
			httpStatus = http.StatusForbidden
			return
		}

		// 初始化虚拟客户端
		cf := client.Cloudflare{
			ZoneID:   Services.Cloudflare.ZoneID,
			APIToken: Services.Cloudflare.APIToken,
			Domain: common.Subdomain{
				A:    instance.DomainRecord.Subdomain.A + "." + instance.DomainRecord.Domain,
				AAAA: instance.DomainRecord.Subdomain.AAAA + "." + instance.DomainRecord.Domain,
			},
		}

		msg, errs = cf.Run(body.Enable, body.IP.IPv4, body.IP.IPv6)
	case common.HuaweiCloud:
		if !Services.HuaweiCloud.Enable {
			httpStatus = http.StatusForbidden
			return
		}

		// 初始化虚拟客户端
		hc := client.HuaweiCloud{
			AccessKeyId:     Services.HuaweiCloud.AccessKeyId,
			SecretAccessKey: Services.HuaweiCloud.SecretAccessKey,
			ZoneName:        instance.DomainRecord.Domain,
			Domain: common.Subdomain{
				A:    instance.DomainRecord.Subdomain.A,
				AAAA: instance.DomainRecord.Subdomain.AAAA,
			},
		}

		msg, errs = hc.Run(body.Enable, body.IP.IPv4, body.IP.IPv6)
	default:
		httpStatus = http.StatusBadRequest
		return
	}

	respBody = common.GeneralResp{}
	for _, v := range msg {
		respBody.Message += v + "\n"
	}
	for _, v := range errs {
		respBody.Message += v.Error() + "\n"
	}
	return
}
