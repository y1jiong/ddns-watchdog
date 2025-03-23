package client

import (
	"ddns-watchdog/internal/common"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	ProjName            = "ddns-watchdog-client"
	NetworkCardFileName = "network_card.json"
)

var (
	ConfDirectoryName = "conf"
	Client            = client{}
	DP                = DNSPod{}
	AD                = AliDNS{}
	Cf                = Cloudflare{}
	HC                = HuaweiCloud{}
)

// AsyncServiceCallback 异步服务回调函数类型
type AsyncServiceCallback func(enabledServices common.Enable, ipv4, ipv6 string) (msg []string, errs []error)

func Install() (err error) {
	if common.IsWindows() {
		return errors.New("windows 暂不支持安装到系统")
	}
	// 注册系统服务
	if Client.CheckCycleMinutes == 0 {
		err = errors.New("设置一下 " + ConfDirectoryName + "/" + ConfFileName + " 的 check_cycle_minutes 吧")
		return
	}
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	serviceContent := []byte(
		"[Unit]\n" +
			"Description=" + ProjName + " Service\n" +
			"After=network-online.target\n\n" +
			"[Service]\n" +
			"Type=simple\n" +
			"WorkingDirectory=" + wd +
			"\nExecStart=" + wd + "/" + ProjName + " -c " + ConfDirectoryName +
			"\nRestart=on-failure\n" +
			"RestartSec=2\n\n" +
			"[Install]\n" +
			"WantedBy=multi-user.target\n",
	)
	if err = os.WriteFile(installPath, serviceContent, 0600); err != nil {
		return
	}
	log.Println("可以使用 systemctl 管理", ProjName, "服务了")
	return
}

func Uninstall() (err error) {
	if common.IsWindows() {
		return errors.New("windows 暂不支持安装到系统")
	}
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	if err = os.Remove(installPath); err != nil {
		return
	}
	log.Println("卸载服务成功")
	log.Println("若要完全删除，请移步到", wd, "和", ConfDirectoryName, "完全删除")
	return
}

func NetworkCardRespond() (map[string]string, error) {
	networkCardInfo := make(map[string]string)

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, i := range interfaces {
		var ipAddr []net.Addr
		ipAddr, err = i.Addrs()
		if err != nil {
			return nil, err
		}
		for j, addrAndMask := range ipAddr {
			// 分离 IP 和子网掩码
			addr := strings.Split(addrAndMask.String(), "/")[0]
			if strings.Contains(addr, ":") {
				addr = common.ExpandIPv6Zero(addr)
			}
			networkCardInfo[i.Name+" "+strconv.Itoa(j)] = addr
		}
	}
	return networkCardInfo, nil
}

func GetOwnIP(enabled common.Enable, apiUrl apiUrl, nc networkCard) (ipv4, ipv6 string, err error) {
	var ncr map[string]string
	// 若需网卡信息，则获取网卡信息并提供给用户
	if nc.Enable && nc.IPv4 == "" && nc.IPv6 == "" {
		ncr, err = NetworkCardRespond()
		if err != nil {
			return
		}
		if err = common.MarshalAndSave(ncr, ConfDirectoryName+"/"+NetworkCardFileName); err != nil {
			return
		}
		err = errors.New("请打开 " + ConfDirectoryName + "/" + NetworkCardFileName + " 选择网卡填入 " +
			ConfDirectoryName + "/" + ConfFileName + " 的 network_card")
		return
	}

	// 若需网卡信息，则获取网卡信息
	if nc.Enable && (nc.IPv4 != "" || nc.IPv6 != "") {
		ncr, err = NetworkCardRespond()
		if err != nil {
			return
		}
	}

	// 启用 IPv4
	if enabled.IPv4 {
		// 启用网卡 IPv4
		if nc.Enable && nc.IPv4 != "" {
			if v, ok := ncr[nc.IPv4]; ok {
				ipv4 = v
			} else {
				err = errors.New("IPv4 选择了不存在的网卡")
			}
		} else {
			// 使用 API 获取 IPv4
			if apiUrl.IPv4 == "" {
				apiUrl.IPv4 = common.DefaultAPIUrl
			}
			var resp *http.Response
			resp, err = http.Get(apiUrl.IPv4)
			if err != nil {
				return
			}
			defer func(Body io.ReadCloser) {
				if t := Body.Close(); t != nil {
					err = t
				}
			}(resp.Body)
			var recvJson []byte
			recvJson, err = io.ReadAll(resp.Body)
			if err != nil {
				return
			}
			var ipInfo common.GetIPResp
			if err = json.Unmarshal(recvJson, &ipInfo); err != nil {
				return
			}
			ipv4 = ipInfo.IP
		}
		if strings.Contains(ipv4, ":") {
			err = errors.New("获取到的 IPv4 格式错误，意外获取到了 " + ipv4)
			ipv4 = ""
		}
	}

	// 启用 IPv6
	if enabled.IPv6 {
		// 启用网卡 IPv6
		if nc.Enable && nc.IPv6 != "" {
			if v, ok := ncr[nc.IPv6]; ok {
				ipv6 = v
			} else {
				err = errors.New("IPv6 选择了不存在的网卡")
			}
		} else {
			// 使用 API 获取 IPv4
			if apiUrl.IPv6 == "" {
				apiUrl.IPv6 = common.DefaultIPv6APIUrl
			}
			var resp *http.Response
			resp, err = http.Get(apiUrl.IPv6)
			if err != nil {
				return
			}
			defer func(Body io.ReadCloser) {
				if t := Body.Close(); t != nil {
					err = t
				}
			}(resp.Body)
			var recvJson []byte
			recvJson, err = io.ReadAll(resp.Body)
			if err != nil {
				return
			}
			var ipInfo common.GetIPResp
			if err = json.Unmarshal(recvJson, &ipInfo); err != nil {
				return
			}
			ipv6 = ipInfo.IP
		}
		if strings.Contains(ipv6, ":") {
			ipv6 = common.ExpandIPv6Zero(ipv6)
		} else {
			err = errors.New("获取到的 IPv6 格式错误，意外获取到了 " + ipv6)
			ipv6 = ""
		}
	}
	return
}
