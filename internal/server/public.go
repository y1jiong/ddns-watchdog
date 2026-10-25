package server

import (
	"crypto/rand"
	"ddns-watchdog/internal/common"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
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

func whitelistPath() string {
	return ConfDir + "/" + WhitelistFilename
}

// fileState returns the modification time and size of path. They are what tells
// the running server that the whitelist was edited from another process.
func fileState(path string) (modTime time.Time, size int64) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	return info.ModTime(), info.Size()
}

// loadWhitelist replaces the in-memory whitelist with the file content; a failed
// read keeps the previous copy. Callers must hold whitelistMu.
func loadWhitelist() (err error) {
	var loaded map[string]whitelistStruct
	if err = common.LoadAndUnmarshal(whitelistPath(), &loaded); err != nil {
		return
	}

	whitelist = loaded
	whitelistModTime, whitelistSize = fileState(whitelistPath())
	return
}

// LoadWhitelist loads the whitelist file at startup.
func LoadWhitelist() (err error) {
	whitelistMu.Lock()
	defer whitelistMu.Unlock()

	return loadWhitelist()
}

// getWhitelistEntry looks a token up after reloading the file when it changed on
// disk, which is what makes a token added or deleted by the CLI take effect
// without restarting the server.
func getWhitelistEntry(token string) (instance whitelistStruct, ok bool) {
	whitelistMu.Lock()
	defer whitelistMu.Unlock()

	if modTime, size := fileState(whitelistPath()); modTime != whitelistModTime || size != whitelistSize {
		if err := loadWhitelist(); err != nil {
			log.Println(err)
			// remember the state even on failure, so a broken file is not re-read
			// and re-logged on every single request
			whitelistModTime, whitelistSize = modTime, size
		}
	}

	instance, ok = whitelist[token]
	return
}

func DelFromWhitelist(token string) (msg string, err error) {
	whitelistMu.Lock()
	defer whitelistMu.Unlock()

	if err = loadWhitelist(); err != nil {
		return
	}
	if _, ok := whitelist[token]; ok {
		msg = fmt.Sprintf("%v has been deleted.\n", whitelist[token].Description)
		delete(whitelist, token)
		if err = common.MarshalAndSave(whitelist, whitelistPath()); err != nil {
			return
		}
		whitelistModTime, whitelistSize = fileState(whitelistPath())
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

	whitelistMu.Lock()
	defer whitelistMu.Unlock()

	if err = loadWhitelist(); err != nil {
		return
	}

	if v, ok := whitelist[token]; ok {
		// every field left empty keeps its stored value
		if domain != "" {
			v.DomainRecord.Domain = domain
		}
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

	if err = common.MarshalAndSave(whitelist, whitelistPath()); err != nil {
		return
	}
	whitelistModTime, whitelistSize = fileState(whitelistPath())
	return
}

func InitWhitelist() (msg string, err error) {
	whitelistMu.Lock()
	defer whitelistMu.Unlock()

	whitelist = make(map[string]whitelistStruct)
	if err = common.MarshalAndSave(whitelist, whitelistPath()); err != nil {
		return
	}
	whitelistModTime, whitelistSize = fileState(whitelistPath())

	return "initialized " + whitelistPath(), nil
}

func GetClientIP(req *http.Request) (ip string) {
	// X-Forwarded-For may be a comma-separated chain; the first value is the original client IP
	ip, _, _ = strings.Cut(req.Header.Get("X-Forwarded-For"), ",")
	if ip == "" {
		ip = req.Header.Get("X-Real-IP")
	}
	if ip == "" && req.RemoteAddr != "" {
		// RemoteAddr is "host:port"; SplitHostPort also strips the brackets
		// around an IPv6 literal.
		if host, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			ip = host
		}
	}
	ip = strings.TrimSpace(ip)

	if strings.Contains(ip, ":") {
		ip = common.ExpandIPv6Zero(ip)
	}
	return
}
