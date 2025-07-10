package common

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	LocalVersion      = "1.5.12"
	DefaultAPIUrl     = "https://yzyweb.cn/ddns-watchdog"
	DefaultIPv6APIUrl = "https://yzyweb.cn/ddns-watchdog6"
	ProjectUrl        = "https://github.com/y1jiong/ddns-watchdog"
)

var (
	GitCommit = ""
	BuildTime = ""
)

// 内容应全小写
const (
	DNSPod      = "dnspod"
	AliDNS      = "alidns"
	Cloudflare  = "cloudflare"
	HuaweiCloud = "huaweicloud"
)

type Enable struct {
	IPv4 bool `json:"ipv4"`
	IPv6 bool `json:"ipv6"`
}

type Subdomain struct {
	A    string `json:"a"`
	AAAA string `json:"aaaa"`
}

type GeneralClient interface {
	Run(Enable, string, string) ([]string, []error)
}

type GetIPResp struct {
	IP      string `json:"ip"`
	Version string `json:"latest_version"`
}

type CenterReq struct {
	Token  string `json:"token"`
	Enable Enable `json:"enable"`
	IP     IPs    `json:"ip"`
}

type IPs struct {
	IPv4 string `json:"ipv4"`
	IPv6 string `json:"ipv6"`
}

type GeneralResp struct {
	Message string `json:"message"`
}

func FormatDirectoryPath(srcPath string) (dstPath string) {
	if length := len(srcPath); srcPath[length-1:] == "/" {
		dstPath = srcPath[0 : length-1]
	} else {
		dstPath = srcPath
	}
	return
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func IsDirExistAndCreate(dirPath string) (err error) {
	_, err = os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(dirPath, 0750)
		}
		return
	}
	return
}

// LoadAndUnmarshal dst 参数要加 & 才能修改原变量
func LoadAndUnmarshal(filePath string, dst any) (err error) {
	_, err = os.Stat(filePath)
	if err != nil {
		return
	}
	jsonContent, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	if err = json.Unmarshal(jsonContent, &dst); err != nil {
		return
	}
	return
}

func MarshalAndSave(content any, filePath string) (err error) {
	if err = IsDirExistAndCreate(filepath.Dir(filePath)); err != nil {
		return
	}
	jsonContent, err := json.MarshalIndent(content, "", "\t")
	if err != nil {
		return
	}
	if err = os.WriteFile(filePath, jsonContent, 0600); err != nil {
		return
	}
	return nil
}

func CompareVersionString(remoteVersion, localVersion string) bool {
	rv := strings.Split(remoteVersion, ".")
	lv := strings.Split(localVersion, ".")

	for i := 0; i < max(len(rv), len(lv)); i++ {
		var r, l int
		if i < len(rv) {
			r, _ = strconv.Atoi(rv[i])
		}
		if i < len(lv) {
			l, _ = strconv.Atoi(lv[i])
		}

		if r > l {
			return true
		}
	}

	return false
}

func ExpandIPv6Zero(ip string) string {
	p := net.ParseIP(ip)
	if p == nil || p.To4() != nil || len(p) != net.IPv6len {
		return ip
	}

	const maxLen = len("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff")
	b := make([]byte, 0, maxLen)

	const hexDigit = "0123456789abcdef"
	appendHex := func(dst []byte, i uint16) []byte {
		if i == 0 {
			return append(dst, '0')
		}
		for j := 3; j >= 0; j-- {
			v := i >> (j * 4)
			if v > 0 {
				dst = append(dst, hexDigit[v&0xf])
			}
		}
		return dst
	}

	for i := 0; i < net.IPv6len; i += 2 {
		if i > 0 {
			b = append(b, ':')
		}
		b = appendHex(b, (uint16(p[i])<<8)|uint16(p[i+1]))
	}
	return string(b)
}

func VersionTips(LatestVersion string) {
	fmt.Println("当前版本", LocalVersion)
	fmt.Println("最新版本", LatestVersion)
	fmt.Println("项目地址", ProjectUrl)
	fmt.Println("Git Commit:", GitCommit)
	fmt.Println("Build Time:", BuildTime)
	switch {
	case strings.Contains(LatestVersion, "N/A"):
		fmt.Println("\n" + LatestVersion + "\n需要手动检查更新，请前往 项目地址 查看")
	case CompareVersionString(LatestVersion, LocalVersion):
		fmt.Println("\n发现新版本，请前往 项目地址 下载")
	}
}
