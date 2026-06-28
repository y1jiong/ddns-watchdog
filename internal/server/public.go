package server

import (
	"crypto/rand"
	"ddns-watchdog/internal/common"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
)

const (
	WhitelistFilename = "whitelist.json"

	InsertSign = "INSERT"
	UpdateSign = "UPDATE"
)

var (
	ConfDir  = "conf"
	Srv      = server{}
	Services = service{}
)

func GenerateToken(length int) (token string) {
	const letter = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	bigInt := new(big.Int).SetInt64(int64(len(letter)))
	b := make([]byte, length)
	for i := range b {
		bigNum, err := rand.Int(rand.Reader, bigInt)
		if err != nil {
			return
		}
		b[i] = letter[bigNum.Int64()]
	}
	return string(b)
}

func DelFromWhitelist(token string) (msg string, err error) {
	if err = common.LoadAndUnmarshal(ConfDir+"/"+WhitelistFilename, &whitelist); err != nil {
		return
	}
	if _, ok := whitelist[token]; ok {
		msg = fmt.Sprintf("%v has been deleted.\n", whitelist[token].Description)
		delete(whitelist, token)
		err = common.MarshalAndSave(whitelist, ConfDir+"/"+WhitelistFilename)
	} else {
		msg = fmt.Sprintf("%v does not exist.\n", token)
	}
	return
}

func AddToWhitelist(token, message, service, domain, a, aaaa string) (status string, err error) {
	if service != "" {
		switch strings.ToLower(service) {
		case common.DNSPod:
			service = common.DNSPod
		case common.AliDNS:
			service = common.AliDNS
		case common.Cloudflare:
			service = common.Cloudflare
		case common.HuaweiCloud:
			service = common.HuaweiCloud
		default:
			err = errors.New("unsupported provider")
			return
		}
	}
	if a == "" && aaaa == "" {
		err = errors.New("no DNS record specified")
		return
	}

	if err = common.LoadAndUnmarshal(ConfDir+"/"+WhitelistFilename, &whitelist); err != nil {
		return
	}

	if v, ok := whitelist[token]; ok {
		if a != "" {
			v.DomainRecord.Subdomain.A = a
		}
		if aaaa != "" {
			v.DomainRecord.Subdomain.AAAA = aaaa
		}
		if service != "" {
			v.Service = service
		}
		if message != "" && message != v.Description {
			v.Description = message
		}
		whitelist[token] = v
		status = UpdateSign
	} else {
		if message == "" {
			message = "undefined"
		}
		// on insert, aaaa defaults to a so a single -A flag covers both record types
		// on update (above branch), we intentionally leave aaaa unchanged if not provided
		if aaaa == "" {
			aaaa = a
		}
		if domain == "" {
			err = errors.New("no domain specified")
		}
		if service == "" {
			err = errors.New("no provider specified")
		}

		if err != nil {
			return
		}

		whitelist[token] = whitelistStruct{
			Enable:      true,
			Description: message,
			Service:     service,
			DomainRecord: domainRecord{
				Domain: domain,
				Subdomain: common.Subdomain{
					A:    a,
					AAAA: aaaa,
				},
			},
		}
		status = InsertSign
	}

	if err = common.MarshalAndSave(whitelist, ConfDir+"/"+WhitelistFilename); err != nil {
		return
	}
	return
}

func InitWhitelist() (msg string, err error) {
	whitelist = make(map[string]whitelistStruct)
	if err = common.MarshalAndSave(whitelist, ConfDir+"/"+WhitelistFilename); err != nil {
		return
	}

	return "initialized " + ConfDir + "/" + WhitelistFilename, nil
}

func LoadWhitelist() (err error) {
	return common.LoadAndUnmarshal(ConfDir+"/"+WhitelistFilename, &whitelist)
}

func GetClientIP(req *http.Request) (ip string) {
	// X-Forwarded-For may be a comma-separated chain; the first value is the original client IP
	ip = req.Header.Get("X-Forwarded-For")
	if idx := strings.IndexByte(ip, ','); idx != -1 {
		ip = ip[:idx]
	}
	if ip == "" {
		ip = req.Header.Get("X-Real-IP")
	}
	if ip == "" && req.RemoteAddr != "" {
		if req.RemoteAddr[0] == '[' {
			if idx := strings.LastIndexByte(req.RemoteAddr, ']'); idx != -1 {
				ip = req.RemoteAddr[1:idx]
			}
		} else {
			if idx := strings.LastIndexByte(req.RemoteAddr, ':'); idx != -1 {
				ip = req.RemoteAddr[:idx]
			}
		}
	}
	ip = strings.TrimSpace(ip)

	if strings.Contains(ip, ":") {
		ip = common.ExpandIPv6Zero(ip)
	}
	return
}
