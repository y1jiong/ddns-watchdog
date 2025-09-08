package client

import (
	"bytes"
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

const NetworkCardFilename = "network_card.json"

var (
	ConfDir = "conf"
	Client  = client{}
	DP      = DNSPod{}
	AD      = AliDNS{}
	Cf      = Cloudflare{}
	HC      = HuaweiCloud{}
)

// ServiceCallback 服务回调函数类型
type ServiceCallback func(enabledServices common.Enable, ipv4, ipv6 string) (msg []string, errs []error)

func Install() (err error) {
	if common.IsWindows() {
		return errors.New("windows 暂不支持安装到系统")
	}
	if Client.CheckCycleMinutes == 0 {
		err = errors.New("设置一下 " + ConfDir + "/" + ConfFilename + " 的 check_cycle_minutes 吧")
		return
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
	log.Println("可以使用 systemctl 管理", projName, "服务了")
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

func NetworkInterfaces() (map[string]string, error) {
	interfaces := make(map[string]string)

	netInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, face := range netInterfaces {
		var ipAddr []net.Addr
		ipAddr, err = face.Addrs()
		if err != nil {
			return nil, err
		}

		for i, addrAndMask := range ipAddr {
			// 分离 IP 和子网掩码
			ip := addrAndMask.String()
			if idx := strings.LastIndexByte(ip, '/'); idx != -1 {
				ip = ip[:idx]
			}
			if strings.Contains(ip, ":") {
				ip = common.ExpandIPv6Zero(ip)
			}
			interfaces[face.Name+" "+strconv.Itoa(i)] = ip
		}
	}
	return interfaces, nil
}

func fallbackIPv6(interfaces map[string]string, preferred string) (string, bool) {
	isPublicUnicast := func(ip string) bool {
		if !strings.Contains(ip, ":") {
			return false
		}
		ipObj := net.ParseIP(ip)
		if ipObj == nil {
			return false
		}
		return ipObj.IsGlobalUnicast() && !ipObj.IsPrivate()
	}

	if preferred != "" {
		// 直接匹配
		if ip, ok := interfaces[preferred]; ok && isPublicUnicast(ip) {
			return ip, true
		}

		// 尝试去掉末尾数字为下一步遍历做准备
		if idx := strings.LastIndexByte(preferred, ' '); idx != -1 {
			if _, err := strconv.Atoi(preferred[idx+1:]); err == nil {
				preferred = preferred[:idx]
			}
		}

		// 尝试遍历匹配 "eth0 0", "eth0 1", ...
		for i := 0; ; i++ {
			ip, ok := interfaces[preferred+" "+strconv.Itoa(i)]
			if !ok {
				break
			}
			if isPublicUnicast(ip) {
				return ip, true
			}
		}
	}

	// 随便找一个
	for _, ip := range interfaces {
		if isPublicUnicast(ip) {
			return ip, true
		}
	}

	return "", false
}

func GetOwnIP(enabled common.Enable, apiUrl apiUrl, nc networkCard, fallback bool) (ipv4, ipv6 string, err error) {
	var interfaces map[string]string
	// 若需网卡信息，则获取网卡信息并提供给用户
	if nc.Enable && nc.IPv4 == "" && nc.IPv6 == "" {
		interfaces, err = NetworkInterfaces()
		if err != nil {
			return
		}

		if err = common.MarshalAndSave(interfaces, ConfDir+"/"+NetworkCardFilename); err != nil {
			return
		}

		err = errors.New("请打开 " + ConfDir + "/" + NetworkCardFilename + " 选择网卡填入 " +
			ConfDir + "/" + ConfFilename + " 的 network_card")
		return
	}

	// 若需网卡信息，则获取网卡信息
	if nc.Enable && (nc.IPv4 != "" || nc.IPv6 != "") {
		interfaces, err = NetworkInterfaces()
		if err != nil {
			return
		}
	}

	// 启用 IPv4
	if enabled.IPv4 {
		// 启用网卡 IPv4
		if nc.Enable && nc.IPv4 != "" {
			if v, ok := interfaces[nc.IPv4]; ok {
				ipv4 = v
			} else {
				err = errors.New("IPv4 选择了不存在的网卡")
				return
			}
		} else {
			// 使用 API 获取 IPv4
			if apiUrl.IPv4 == "" {
				apiUrl.IPv4 = common.DefaultAPIUrl
			}

			var resp *http.Response
			resp, err = httpGet(apiUrl.IPv4)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			var respJson []byte
			respJson, err = io.ReadAll(resp.Body)
			if err != nil {
				return
			}

			var ipInfo common.GetIPResp
			if err = json.Unmarshal(respJson, &ipInfo); err != nil {
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
			var (
				v  string
				ok bool
			)
			if fallback {
				v, ok = fallbackIPv6(interfaces, nc.IPv6)
			} else {
				v, ok = interfaces[nc.IPv6]
			}
			if ok {
				ipv6 = v
			} else {
				err = errors.New("IPv6 选择了不存在的网卡")
				return
			}
		} else {
			// 使用 API 获取 IPv6
			if apiUrl.IPv6 == "" {
				apiUrl.IPv6 = common.DefaultIPv6APIUrl
			}

			var resp *http.Response
			resp, err = httpGet(apiUrl.IPv6)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			var respJson []byte
			respJson, err = io.ReadAll(resp.Body)
			if err != nil {
				return
			}

			var ipInfo common.GetIPResp
			if err = json.Unmarshal(respJson, &ipInfo); err != nil {
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

func AccessCenter(ipv4, ipv6 string) {
	// 构造请求 body
	reqBody := common.CenterReq{
		Token:  Client.Center.Token,
		Enable: Client.Enable,
		IP: common.IPs{
			IPv4: ipv4,
			IPv6: ipv6,
		},
	}

	reqJson, err := json.Marshal(reqBody)
	if err != nil {
		log.Println(err)
		return
	}

	// 发送请求
	req, err := httpNewRequest(http.MethodPost, Client.Center.APIUrl, bytes.NewReader(reqJson))
	if err != nil {
		log.Println(err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := common.DefaultHttpClient.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	// 处理结果
	if resp.StatusCode != http.StatusOK {
		log.Println("The status code returned by the center is", resp.StatusCode)
	}

	respBodyJson, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}
	if len(respBodyJson) == 0 {
		return
	}

	var respBody common.GeneralResp
	if err = json.Unmarshal(respBodyJson, &respBody); err != nil {
		log.Println(err)
		return
	}
	for _, v := range strings.Split(respBody.Message, "\n") {
		if v != "" {
			log.Println(v)
		}
	}
}
