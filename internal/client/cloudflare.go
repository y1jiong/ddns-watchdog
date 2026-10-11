package client

import (
	"bytes"
	"ddns-watchdog/internal/common"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/bitly/go-simplejson"
)

const (
	CloudflareConfFilename = "cloudflare.json"
	cloudflarePrefix       = "Cloudflare: "
)

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
		ZoneID:   "zone ID from the right sidebar of your domain page",
		APIToken: "get from https://dash.cloudflare.com/profile/api-tokens",
		Domain: common.Subdomain{
			A:    "subdomain.example.com",
			AAAA: "subdomain.example.com",
		},
	}

	return "initialized " + ConfDir + "/" + CloudflareConfFilename,
		common.MarshalAndSave(cfc, ConfDir+"/"+CloudflareConfFilename)
}

func (cfc *Cloudflare) LoadConf() (err error) {
	// A missing file is allowed: the provider may be configured through
	// environment variables alone. missingFile keeps the stat error so that a
	// config which is still empty afterward reports the missing file rather than
	// a list of unset fields.
	var missingFile error
	if err = common.LoadAndUnmarshal(ConfDir+"/"+CloudflareConfFilename, &cfc); err != nil {
		if !os.IsNotExist(err) {
			return
		}
		missingFile = err
	}
	err = nil

	if v := os.Getenv("DDNS_CLOUDFLARE_ZONE_ID"); v != "" {
		cfc.ZoneID = v
	}
	if v := os.Getenv("DDNS_CLOUDFLARE_TOKEN"); v != "" {
		cfc.APIToken = v
	}
	if v := os.Getenv("DDNS_CLOUDFLARE_DOMAIN_A"); v != "" {
		cfc.Domain.A = v
	}
	if v := os.Getenv("DDNS_CLOUDFLARE_DOMAIN_AAAA"); v != "" {
		cfc.Domain.AAAA = v
	}
	if v := os.Getenv("DDNS_CLOUDFLARE_PROXIED"); v != "" {
		cfc.Proxied = v == "true" || v == "1"
	}

	if cfc.ZoneID == "" || cfc.APIToken == "" || (cfc.Domain.A == "" && cfc.Domain.AAAA == "") {
		if missingFile != nil {
			return missingFile
		}
		return errors.New("check zone_id, api_token, domain in " + ConfDir + "/" + CloudflareConfFilename)
	}
	return
}

func (cfc *Cloudflare) Run(enabled common.Enable, ipv4, ipv6 string) (msg []string, errs []error) {
	if ipv4 != "" && enabled.IPv4 && cfc.Domain.A != "" {
		domainId, recordIP, err := cfc.getParseRecord(cfc.Domain.A, "A")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv4 {
			if err = cfc.updateParseRecord(ipv4, domainId, "A", cfc.Domain.A); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, cloudflarePrefix+cfc.Domain.A+" record updated to "+ipv4)
			}
		}
	}
	if ipv6 != "" && enabled.IPv6 && cfc.Domain.AAAA != "" {
		domainId, recordIP, err := cfc.getParseRecord(cfc.Domain.AAAA, "AAAA")
		if err != nil {
			errs = append(errs, err)
		} else if recordIP != ipv6 {
			if err = cfc.updateParseRecord(ipv6, domainId, "AAAA", cfc.Domain.AAAA); err != nil {
				errs = append(errs, err)
			} else {
				msg = append(msg, cloudflarePrefix+cfc.Domain.AAAA+" record updated to "+ipv6)
			}
		}
	}
	return
}

func (cfc *Cloudflare) getParseRecord(domain, recordType string) (domainId, recordIP string, err error) {
	url := "https://api.cloudflare.com/client/v4/zones/" + cfc.ZoneID + "/dns_records?name=" + domain
	req, err := httpNewRequest(http.MethodGet, url, nil)
	if err != nil {
		return
	}

	req.Header.Set("Authorization", "Bearer "+cfc.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := common.DefaultHttpClient.Do(req)
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
		err = errors.New(cloudflarePrefix + errMsg)
		return
	}
	if !jsonObj.Get("success").MustBool() {
		err = errors.New(cloudflarePrefix + "authentication failed")
		return
	}

	records, err := jsonObj.Get("result").Array()
	if len(records) == 0 {
		err = errors.New(cloudflarePrefix + domain + " record not found")
		return
	}

	for _, v := range records {
		element, isObject := v.(map[string]any)
		if !isObject {
			continue
		}

		name, ok := mapString(element, "name")
		if !ok || name != domain {
			continue
		}
		elementType, ok := mapString(element, "type")
		if !ok || elementType != recordType {
			continue
		}

		domainId, _ = mapString(element, "id")
		recordIP, _ = mapString(element, "content")
		break
	}

	if domainId == "" || recordIP == "" {
		err = errors.New(cloudflarePrefix + domain + " " + recordType + " record not found")
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

	req, err := httpNewRequest(http.MethodPut, url, bytes.NewReader(reqJson))
	if err != nil {
		return
	}

	req.Header.Set("Authorization", "Bearer "+cfc.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := common.DefaultHttpClient.Do(req)
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
			element, isObject := value.(map[string]any)
			if !isObject {
				continue
			}

			code := ""
			switch v := element["code"].(type) {
			case json.Number:
				code = v.String()
			case string:
				code = v
			}
			if code != "" {
				code += ": "
			}
			errMsg, _ := mapString(element, "message")
			errorsMsg = errorsMsg + cloudflarePrefix + code + errMsg + "\n"
		}

		err = errors.New(errorsMsg)
	}
	return
}
