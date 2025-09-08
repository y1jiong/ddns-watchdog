package server

import (
	"crypto/rand"
	"ddns-watchdog/internal/common"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
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
		// 规范输入
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
			err = errors.New("不支持的服务供应商")
			return
		}
	}
	if a == "" && aaaa == "" {
		err = errors.New("没有指定解析记录")
		return
	}

	// 加载白名单
	if err = common.LoadAndUnmarshal(ConfDir+"/"+WhitelistFilename, &whitelist); err != nil {
		return
	}

	// 是否已经存在记录
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
		if aaaa == "" {
			aaaa = a
		}
		if domain == "" {
			err = errors.New("没有指定需要操作的域名")
		}
		if service == "" {
			err = errors.New("没有指定需要采用的服务供应商")
		}

		if err != nil {
			return
		}

		// 写入白名单
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

	// 保存白名单
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

	return "初始化 " + ConfDir + "/" + WhitelistFilename, nil
}

func LoadWhitelist() (err error) {
	return common.LoadAndUnmarshal(ConfDir+"/"+WhitelistFilename, &whitelist)
}

func GetClientIP(req *http.Request) (ip string) {
	ip = req.Header.Get("X-Forwarded-For")
	if idx := strings.IndexByte(ip, ','); idx != -1 {
		ip = ip[:idx]
	}
	if ip == "" {
		ip = req.Header.Get("X-Real-IP")
	}
	if ip == "" && req.RemoteAddr != "" {
		// 只保留 ip:port 的 ip
		if req.RemoteAddr[0] == '[' {
			// IPv6
			if idx := strings.LastIndexByte(req.RemoteAddr, ']'); idx != -1 {
				ip = req.RemoteAddr[1:idx]
			}
		} else {
			// IPv4
			if idx := strings.LastIndexByte(req.RemoteAddr, ':'); idx != -1 {
				ip = req.RemoteAddr[:idx]
			}
		}
	}
	ip = strings.TrimSpace(ip)

	// IPv6 地址展开
	if strings.Contains(ip, ":") {
		ip = common.ExpandIPv6Zero(ip)
	}
	return
}

func Install() (err error) {
	if common.IsWindows() {
		return errors.New("windows 暂不支持安装到系统")
	}

	wd, err := os.Getwd()
	if err != nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}

	serviceContent := []byte(
		"[Unit]\n" +
			"Description=" + projName + " Service\n" +
			"Wants=network-online.target\n" +
			"After=network-online.target\n\n" +
			"[Service]\n" +
			"Type=simple\n" +
			"WorkingDirectory=" + wd +
			"\nExecStart=" + exe + " -c " + ConfDir +
			"\nRestart=on-failure\n" +
			"RestartSec=2\n" +
			"LimitNOFILE=65535\n\n" +
			"[Install]\n" +
			"WantedBy=multi-user.target\n",
	)
	if err = os.WriteFile(installPath, serviceContent, 0o644); err != nil {
		return
	}
	log.Println("可以使用 systemctl 控制", projName, "服务了")
	return
}

func Uninstall() (err error) {
	if common.IsWindows() {
		return errors.New("windows 暂不支持安装到系统")
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}

	if err = os.Remove(installPath); err != nil {
		return
	}
	log.Println("卸载服务成功\n若要完全删除，请移步到", exe, "和", ConfDir, "完全删除")
	return
}
