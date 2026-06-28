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

type ServiceCallback func(enabledServices common.Enable, ipv4, ipv6 string) (msg []string, errs []error)

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
			ip := addrAndMask.String()
			if idx := strings.LastIndexByte(ip, '/'); idx != -1 {
				ip = ip[:idx]
			}
			if strings.Contains(ip, ":") {
				ip = common.ExpandIPv6Zero(ip)
			}
			// key format: "<ifname> <addr-index>", e.g. "eth0 0", "eth0 1"
			// the index makes each address unique; fallbackIPv6 strips it to search across addresses of the same NIC
			interfaces[face.Name+" "+strconv.Itoa(i)] = ip
		}
	}
	return interfaces, nil
}

// fallbackIPv6 finds a public unicast IPv6 address. It tries preferred first, then iterates
// other addresses on the same NIC ("eth0 0" → "eth0 1" → …), then scans all interfaces.
// Used when the configured NIC may not always have a public IPv6 (e.g. during renumbering).
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
		if ip, ok := interfaces[preferred]; ok && isPublicUnicast(ip) {
			return ip, true
		}

		if idx := strings.LastIndexByte(preferred, ' '); idx != -1 {
			if _, err := strconv.Atoi(preferred[idx+1:]); err == nil {
				preferred = preferred[:idx]
			}
		}

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

	for _, ip := range interfaces {
		if isPublicUnicast(ip) {
			return ip, true
		}
	}

	return "", false
}

func GetOwnIP(enabled common.Enable, apiUrl apiUrl, nc networkCard, fallback bool) (ipv4, ipv6 string, err error) {
	var interfaces map[string]string
	if nc.Enable && nc.IPv4 == "" && nc.IPv6 == "" {
		interfaces, err = NetworkInterfaces()
		if err != nil {
			return
		}

		if err = common.MarshalAndSave(interfaces, ConfDir+"/"+NetworkCardFilename); err != nil {
			return
		}

		err = errors.New("open " + ConfDir + "/" + NetworkCardFilename +
			" to select a network card and set it in " + ConfDir + "/" + ConfFilename)
		return
	}

	if nc.Enable && (nc.IPv4 != "" || nc.IPv6 != "") {
		interfaces, err = NetworkInterfaces()
		if err != nil {
			return
		}
	}

	if enabled.IPv4 {
		if nc.Enable && nc.IPv4 != "" {
			if v, ok := interfaces[nc.IPv4]; ok {
				ipv4 = v
			} else {
				err = errors.New("ipv4: selected network card does not exist")
				return
			}
		} else {
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

		// API returned a colon address — the caller may have pointed IPv4 URL at an IPv6 endpoint
		if strings.Contains(ipv4, ":") {
			err = errors.New("unexpected ipv4 format: " + ipv4)
			ipv4 = ""
		}
	}

	if enabled.IPv6 {
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
				err = errors.New("ipv6: selected network card does not exist")
				return
			}
		} else {
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

		// colon presence is how we distinguish a real IPv6 address from an IPv4 returned by the wrong endpoint
		if strings.Contains(ipv6, ":") {
			ipv6 = common.ExpandIPv6Zero(ipv6)
		} else {
			err = errors.New("unexpected ipv6 format: " + ipv6)
			ipv6 = ""
		}
	}
	return
}

func AccessCenter(ipv4, ipv6 string) {
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
