package client

import (
	"crypto/hmac"
	"crypto/sha1"
	"ddns-watchdog/internal/common"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	AliDNSConfFilename = "alidns.json"
	aliDNSPrefix       = "AliDNS: "
	aliDNSEndpoint     = "https://alidns.aliyuncs.com/"
	aliDNSAPIVersion   = "2015-01-09"
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
	c := aliDNSClient{akID: ad.AccessKeyId, akSec: ad.AccessKeySecret}

	if ipv4 != "" && enabled.IPv4 && ad.SubDomain.A != "" {
		recordId, recordIP, err := ad.getRecord(c, ad.SubDomain.A, "A")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv4 {
			if err = c.updateRecord(recordId, ad.SubDomain.A, "A", ipv4); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, aliDNSPrefix+ad.SubDomain.A+"."+ad.Domain+" record updated to "+ipv4)
			}
		}
	}
	if ipv6 != "" && enabled.IPv6 && ad.SubDomain.AAAA != "" {
		recordId, recordIP, err := ad.getRecord(c, ad.SubDomain.AAAA, "AAAA")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv6 {
			if err = c.updateRecord(recordId, ad.SubDomain.AAAA, "AAAA", ipv6); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, aliDNSPrefix+ad.SubDomain.AAAA+"."+ad.Domain+" record updated to "+ipv6)
			}
		}
	}
	return
}

func (ad *AliDNS) getRecord(c aliDNSClient, subDomain, recordType string) (recordId, recordIP string, err error) {
	body, err := c.call(map[string]string{
		"Action":     "DescribeDomainRecords",
		"DomainName": ad.Domain,
		"PageSize":   "500",
	})
	if err != nil {
		return
	}

	var resp struct {
		DomainRecords struct {
			Record []struct {
				RecordId string
				RR       string
				Type     string
				Value    string
			}
		}
		Code    string
		Message string
	}
	if err = json.Unmarshal(body, &resp); err != nil {
		return
	}
	if resp.Code != "" {
		err = errors.New(aliDNSPrefix + resp.Code + ": " + resp.Message)
		return
	}

	for _, r := range resp.DomainRecords.Record {
		if r.RR == subDomain && r.Type == recordType {
			recordId = r.RecordId
			recordIP = r.Value
			return
		}
	}
	err = errors.New(aliDNSPrefix + subDomain + "." + ad.Domain + " " + recordType + " record not found")
	return
}

// --- minimal AliDNS RPC client ---

type aliDNSClient struct {
	akID  string
	akSec string
}

func (c aliDNSClient) updateRecord(recordId, rr, recordType, value string) error {
	body, err := c.call(map[string]string{
		"Action":   "UpdateDomainRecord",
		"RecordId": recordId,
		"RR":       rr,
		"Type":     recordType,
		"Value":    value,
	})
	if err != nil {
		return err
	}
	var resp struct {
		Code    string
		Message string
	}
	if err = json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Code != "" {
		return errors.New(aliDNSPrefix + resp.Code + ": " + resp.Message)
	}
	return nil
}

func (c aliDNSClient) call(params map[string]string) ([]byte, error) {
	params["Format"] = "JSON"
	params["Version"] = aliDNSAPIVersion
	params["AccessKeyId"] = c.akID
	params["SignatureMethod"] = "HMAC-SHA1"
	params["SignatureVersion"] = "1.0"
	params["SignatureNonce"] = fmt.Sprintf("%016x", rand.Uint64())
	params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, len(keys))
	for i, k := range keys {
		pairs[i] = aliEncode(k) + "=" + aliEncode(params[k])
	}
	query := strings.Join(pairs, "&")

	mac := hmac.New(sha1.New, []byte(c.akSec+"&"))
	mac.Write([]byte("GET&%2F&" + aliEncode(query)))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	resp, err := common.DefaultHttpClient.Get(aliDNSEndpoint + "?" + query + "&Signature=" + aliEncode(sig))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// aliEncode is Alibaba Cloud's percent-encoding: spaces become %20 (not +), ~ is left unencoded.
func aliEncode(s string) string {
	e := url.QueryEscape(s)
	e = strings.ReplaceAll(e, "+", "%20")
	e = strings.ReplaceAll(e, "%7E", "~")
	return e
}
