package server

import (
	"ddns-watchdog/internal/client"
	"ddns-watchdog/internal/common"
	"io"
	"net/http"
	"sync"
	"time"
)

const projName = "ddns-watchdog-server"

var (
	whitelist map[string]whitelistStruct

	// whitelistMu guards whitelist and the file state below. The running server
	// reloads the file when it changes on disk, so "ddns-watchdog-server -a/-d"
	// takes effect without restarting it.
	whitelistMu      sync.Mutex
	whitelistModTime time.Time
	whitelistSize    int64
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

// doVirtualClient runs the provider configured for instance and returns the
// status to report to the client. A provider error is reported as 502 with the
// reason in the body, so the client knows the records were not updated.
func doVirtualClient(body common.CenterReq, instance whitelistStruct) (httpStatus int, respBody common.GeneralResp) {
	var (
		msg  []string
		errs []error
	)

	switch instance.Service {
	case common.DNSPod:
		if !Services.DNSPod.Enable {
			return http.StatusForbidden, common.GeneralResp{Message: common.DNSPod + " is not enabled on this server"}
		}

		dp := client.DNSPod{
			ID:        Services.DNSPod.ID,
			Token:     Services.DNSPod.Token,
			Domain:    instance.DomainRecord.Domain,
			SubDomain: instance.DomainRecord.Subdomain,
		}

		msg, errs = dp.Run(body.Enable, body.IP.IPv4, body.IP.IPv6)
	case common.AliDNS:
		if !Services.AliDNS.Enable {
			return http.StatusForbidden, common.GeneralResp{Message: common.AliDNS + " is not enabled on this server"}
		}

		ad := client.AliDNS{
			AccessKeyId:     Services.AliDNS.AccessKeyId,
			AccessKeySecret: Services.AliDNS.AccessKeySecret,
			Domain:          instance.DomainRecord.Domain,
			SubDomain:       instance.DomainRecord.Subdomain,
		}

		msg, errs = ad.Run(body.Enable, body.IP.IPv4, body.IP.IPv6)
	case common.Cloudflare:
		if !Services.Cloudflare.Enable {
			return http.StatusForbidden, common.GeneralResp{Message: common.Cloudflare + " is not enabled on this server"}
		}

		// Cloudflare expects full domain names in its record API, unlike other providers that split subdomain + domain
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
			return http.StatusForbidden, common.GeneralResp{Message: common.HuaweiCloud + " is not enabled on this server"}
		}

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
		return http.StatusBadRequest, common.GeneralResp{Message: "unsupported provider: " + instance.Service}
	}

	return centerResp(msg, errs)
}

// centerResp turns a provider run into the status to report to the client: 502
// when the provider failed, so the client knows the records were not updated and
// retries on its next cycle.
func centerResp(msg []string, errs []error) (httpStatus int, respBody common.GeneralResp) {
	for _, v := range msg {
		respBody.Message += v + "\n"
	}
	for _, v := range errs {
		respBody.Message += v.Error() + "\n"
	}

	httpStatus = http.StatusOK
	if len(errs) != 0 {
		httpStatus = http.StatusBadGateway
	}
	return
}

func httpGet(url string) (*http.Response, error) {
	req, err := httpNewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return common.DefaultHttpClient.Do(req)
}

func httpNewRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", projName+"/"+common.Version)
	return req, nil
}
