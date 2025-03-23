package main

import (
	"bytes"
	"crypto/tls"
	"ddns-watchdog/internal/client"
	"ddns-watchdog/internal/common"
	"encoding/json"
	"errors"
	"fmt"
	flag "github.com/spf13/pflag"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	installOption   = flag.BoolP("install", "I", false, "安装服务并退出")
	uninstallOption = flag.BoolP("uninstall", "U", false, "卸载服务并退出")
	enforcement     = flag.BoolP("force", "f", false, "强制检查 DNS 解析记录")
	version         = flag.BoolP("version", "V", false, "查看当前版本并检查更新后退出")
	initOption      = flag.StringP("init", "i", "", "有选择地初始化配置文件并退出，可以组合使用 (例 01)\n"+
		"0 -> "+client.ConfFileName+"\n"+
		"1 -> "+client.DNSPodConfFileName+"\n"+
		"2 -> "+client.AliDNSConfFileName+"\n"+
		"3 -> "+client.CloudflareConfFileName+"\n"+
		"4 -> "+client.HuaweiCloudConfFileName)
	confPath             = flag.StringP("conf", "c", "", "指定配置文件目录 (目录有空格请放在双引号中间)")
	printNetworkCardInfo = flag.BoolP("network-card", "n", false, "输出网卡信息并退出")
)

func main() {
	// 处理 flag
	exit, err := processFlag()
	if err != nil {
		log.Fatal(err)
	}
	if exit {
		return
	}

	// 加载服务配置
	if err = loadConf(); err != nil {
		log.Fatal(err)
	}

	// 周期循环
	if client.Client.CheckCycleMinutes <= 0 {
		check()
	} else {
		cycle := time.NewTicker(time.Duration(client.Client.CheckCycleMinutes) * time.Minute)
		for {
			check()
			<-cycle.C
		}
	}
}

func processFlag() (exit bool, err error) {
	flag.Parse()
	// 打印网卡信息
	if *printNetworkCardInfo {
		var ncr map[string]string
		ncr, err = client.NetworkCardRespond()
		if err != nil {
			return
		}
		var arr []string
		for key := range ncr {
			arr = append(arr, key)
		}
		sort.Strings(arr)
		for _, key := range arr {
			fmt.Printf("%v\n\t%v\n", key, ncr[key])
		}
		return true, nil
	}

	// 加载自定义配置文件目录
	if *confPath != "" {
		client.ConfDirectoryName = common.FormatDirectoryPath(*confPath)
	}

	// 有选择地初始化配置文件
	if *initOption != "" {
		for _, event := range *initOption {
			if err = initConf(string(event)); err != nil {
				return
			}
		}
		return true, nil
	}

	// 加载客户端配置
	// 不得不放在这个地方，因为有下面的检查版本和安装 / 卸载服务
	if err = client.Client.LoadConf(); err != nil {
		return
	}

	// 检查版本
	if *version {
		client.Client.CheckLatestVersion()
		return true, nil
	}

	// 安装 / 卸载服务
	switch {
	case *installOption:
		return true, client.Install()
	case *uninstallOption:
		return true, client.Uninstall()
	}
	return
}

func initConf(event string) (err error) {
	msg := ""
	switch event {
	case "0":
		msg, err = client.Client.InitConf()
	case "1":
		msg, err = client.DP.InitConf()
	case "2":
		msg, err = client.AD.InitConf()
	case "3":
		msg, err = client.Cf.InitConf()
	case "4":
		msg, err = client.HC.InitConf()
	default:
		err = errors.New("你初始化了一个寂寞")
	}
	if err != nil {
		return
	}
	log.Println(msg)
	return
}

func loadConf() (err error) {
	if !client.Client.Center.Enable {
		if client.Client.Services.DNSPod {
			if err = client.DP.LoadConf(); err != nil {
				return
			}
		}
		if client.Client.Services.AliDNS {
			if err = client.AD.LoadConf(); err != nil {
				return
			}
		}
		if client.Client.Services.Cloudflare {
			if err = client.Cf.LoadConf(); err != nil {
				return
			}
		}
		if client.Client.Services.HuaweiCloud {
			if err = client.HC.LoadConf(); err != nil {
				return
			}
		}
	}
	return
}

func check() {
	// 获取 IP
	ipv4, ipv6, err := client.GetOwnIP(client.Client.Enable, client.Client.APIUrl, client.Client.NetworkCard)
	if err != nil {
		log.Println(err)
		if ipv4 == "" && ipv6 == "" {
			return
		}
	}

	// 进入更新流程
	if ipv4 != client.Client.LatestIPv4 || ipv6 != client.Client.LatestIPv6 || *enforcement {
		if ipv4 != client.Client.LatestIPv4 {
			client.Client.LatestIPv4 = ipv4
		}
		if ipv6 != client.Client.LatestIPv6 {
			client.Client.LatestIPv6 = ipv6
		}
		var wg = sync.WaitGroup{}
		if client.Client.Center.Enable {
			accessCenter(ipv4, ipv6)
		} else {
			if client.Client.Services.DNSPod {
				wg.Add(1)
				go asyncServiceInterface(ipv4, ipv6, client.DP.Run, &wg)
			}
			if client.Client.Services.AliDNS {
				wg.Add(1)
				go asyncServiceInterface(ipv4, ipv6, client.AD.Run, &wg)
			}
			if client.Client.Services.Cloudflare {
				wg.Add(1)
				go asyncServiceInterface(ipv4, ipv6, client.Cf.Run, &wg)
			}
			if client.Client.Services.HuaweiCloud {
				wg.Add(1)
				go asyncServiceInterface(ipv4, ipv6, client.HC.Run, &wg)
			}
		}
		wg.Wait()
	}
}

func asyncServiceInterface(ipv4, ipv6 string, callback client.AsyncServiceCallback, wg *sync.WaitGroup) {
	defer wg.Done()
	msg, err := callback(client.Client.Enable, ipv4, ipv6)
	for _, row := range err {
		log.Println(row)
	}
	for _, row := range msg {
		log.Println(row)
	}
}

func accessCenter(ipv4, ipv6 string) {
	// 创建 http 客户端
	hc := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	// 构造请求 body
	reqBody := common.CenterReq{
		Token:  client.Client.Center.Token,
		Enable: client.Client.Enable,
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
	req, err := http.NewRequest(http.MethodPost, client.Client.Center.APIUrl, bytes.NewReader(reqJson))
	if err != nil {
		log.Println(err)
		return
	}
	resp, err := hc.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer func(Body io.ReadCloser) {
		if t := Body.Close(); t != nil {
			err = t
		}
	}(resp.Body)

	// 处理结果
	if resp.StatusCode != http.StatusOK {
		log.Println("The status code returned by the center is", resp.StatusCode)
	}
	respBodyJson, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}
	if len(respBodyJson) > 0 {
		var respBody = common.GeneralResp{}
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
}
