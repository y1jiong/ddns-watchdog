package main

import (
	"ddns-watchdog/internal/client"
	intsvc "ddns-watchdog/internal/service"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	flag "github.com/spf13/pflag"
)

const (
	svcName        = "ddns-watchdog-client"
	svcDisplayName = "DDNS Watchdog Client"
	svcDescription = "Dynamic DNS update client"
)

var (
	confDir              = flag.StringP("conf", "c", "", "指定配置文件目录 (目录有空格请放在双引号中间)")
	installOption        = flag.BoolP("install", "I", false, "安装服务并退出")
	uninstallOption      = flag.BoolP("uninstall", "U", false, "卸载服务并退出")
	enforcement          = flag.BoolP("force", "f", false, "强制检查 DNS 解析记录")
	version              = flag.BoolP("version", "V", false, "查看当前版本并检查更新后退出")
	initOption           = flag.StringP("init", "i", "", "有选择地初始化配置文件并退出，可以组合使用 (例 01)\n"+
		"0 -> "+client.ConfFilename+"\n"+
		"1 -> "+client.DNSPodConfFilename+"\n"+
		"2 -> "+client.AliDNSConfFilename+"\n"+
		"3 -> "+client.CloudflareConfFilename+"\n"+
		"4 -> "+client.HuaweiCloudConfFilename)
	printNetworkCardInfo = flag.BoolP("network-card", "n", false, "输出网卡信息并退出")
)

func main() {
	// Detect and strip positional args before pflag sees them.
	var (
		isRunMode     bool
		serviceAction string
	)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "run":
			isRunMode = true
			os.Args = append(os.Args[:1], os.Args[2:]...)
		case "service":
			if len(os.Args) > 2 {
				serviceAction = os.Args[2]
				os.Args = append(os.Args[:1], os.Args[3:]...)
			}
		}
	}

	flag.Parse()

	if *confDir != "" {
		client.ConfDir = filepath.Clean(*confDir)
	}

	// "service <action>" subcommand (v2 addition)
	if serviceAction != "" {
		svc, err := newService()
		if err != nil {
			log.Fatal(err)
		}
		intsvc.RunCommand(svc, serviceAction)
		return
	}

	// "run" positional arg: invoked by the OS service manager
	if isRunMode {
		svc, err := newService()
		if err != nil {
			log.Fatal(err)
		}
		intsvc.Run(svc)
		return
	}

	// Standard flag-based flow (backward compatible with v1)
	exit, err := processFlag()
	if err != nil {
		log.Fatal(err)
	}
	if exit {
		return
	}

	if err = loadConf(); err != nil {
		log.Fatal(err)
	}

	runLoop()
}

func newService() (*intsvc.Svc, error) {
	args := []string{"-c", absConfDir()}
	return intsvc.New(svcName, svcDisplayName, svcDescription, args, startDaemon)
}

func startDaemon() {
	if err := client.Client.LoadConf(); err != nil {
		log.Fatal(err)
	}
	if err := loadConf(); err != nil {
		log.Fatal(err)
	}
	runLoop()
}

func absConfDir() string {
	dir := client.ConfDir
	if !filepath.IsAbs(dir) {
		if wd, err := os.Getwd(); err == nil {
			dir = filepath.Join(wd, dir)
		}
	}
	return dir
}

func processFlag() (exit bool, err error) {
	// flag.Parse() already called in main()

	if *printNetworkCardInfo {
		var interfaces map[string]string
		interfaces, err = client.NetworkInterfaces()
		if err != nil {
			return
		}

		arr := make([]string, 0, len(interfaces))
		for name := range interfaces {
			arr = append(arr, name)
		}
		sort.Strings(arr)

		for _, name := range arr {
			fmt.Printf("%v\n\t%v\n", name, interfaces[name])
		}
		return true, nil
	}

	if *initOption != "" {
		for _, event := range *initOption {
			if err = initConf(string(event)); err != nil {
				return
			}
		}
		return true, nil
	}

	if err = client.Client.LoadConf(); err != nil {
		return
	}

	if *version {
		client.Client.CheckLatestVersion()
		return true, nil
	}

	switch {
	case *installOption:
		svc, e := newService()
		if e != nil {
			return true, e
		}
		intsvc.RunCommand(svc, "install")
		return true, nil
	case *uninstallOption:
		svc, e := newService()
		if e != nil {
			return true, e
		}
		intsvc.RunCommand(svc, "uninstall")
		return true, nil
	}
	return
}

func initConf(event string) (err error) {
	var msg string
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
	if client.Client.Center.Enable {
		return
	}

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
	return
}

func runLoop() {
	if client.Client.CheckCycleMinutes <= 0 {
		check()
		return
	}

	cycle := time.NewTicker(time.Duration(client.Client.CheckCycleMinutes) * time.Minute)
	for {
		check()
		<-cycle.C
	}
}

func check() {
	ipv4, ipv6, err := client.GetOwnIP(client.Client.Enable, client.Client.APIUrl, client.Client.NetworkCard, client.Client.EnableIPv6Fallback)
	if err != nil {
		log.Println(err)
		if ipv4 == "" && ipv6 == "" {
			return
		}
	}

	if ipv4 == client.Client.LatestIPv4 && ipv6 == client.Client.LatestIPv6 && !*enforcement {
		return
	}

	if ipv4 != client.Client.LatestIPv4 {
		client.Client.LatestIPv4 = ipv4
	}
	if ipv6 != client.Client.LatestIPv6 {
		client.Client.LatestIPv6 = ipv6
	}

	if client.Client.Center.Enable {
		client.AccessCenter(ipv4, ipv6)
		return
	}

	wg := sync.WaitGroup{}
	defer wg.Wait()
	if client.Client.Services.DNSPod {
		wg.Go(func() { serviceInterface(ipv4, ipv6, client.DP.Run) })
	}
	if client.Client.Services.AliDNS {
		wg.Go(func() { serviceInterface(ipv4, ipv6, client.AD.Run) })
	}
	if client.Client.Services.Cloudflare {
		wg.Go(func() { serviceInterface(ipv4, ipv6, client.Cf.Run) })
	}
	if client.Client.Services.HuaweiCloud {
		wg.Go(func() { serviceInterface(ipv4, ipv6, client.HC.Run) })
	}
}

func serviceInterface(ipv4, ipv6 string, callback client.ServiceCallback) {
	msg, errs := callback(client.Client.Enable, ipv4, ipv6)
	for _, row := range errs {
		log.Println(row)
	}
	for _, row := range msg {
		log.Println(row)
	}
}
