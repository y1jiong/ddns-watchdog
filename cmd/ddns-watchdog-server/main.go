package main

import (
	"crypto/tls"
	"ddns-watchdog/internal/common"
	"ddns-watchdog/internal/server"
	intsvc "ddns-watchdog/internal/service"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	flag "github.com/spf13/pflag"
)

const (
	svcName        = "ddns-watchdog-server"
	svcDisplayName = "DDNS Watchdog Server"
	svcDescription = "Dynamic DNS watchdog IP echo and center proxy server"
)

var (
	confDir         = flag.StringP("conf", "c", "", "指定配置文件目录 (目录有空格请放在双引号中间)")
	installOption   = flag.BoolP("install", "I", false, "安装服务并退出")
	uninstallOption = flag.BoolP("uninstall", "U", false, "卸载服务并退出")
	version         = flag.BoolP("version", "V", false, "查看当前版本并检查更新后退出")
	initOption      = flag.StringP("init", "i", "", "有选择地初始化配置文件并退出，可以组合使用 (例 01)\n"+
		"0 -> "+server.ConfFilename+"\n"+
		"1 -> "+server.ServiceConfFilename+"\n"+
		"2 -> "+server.WhitelistFilename)
	add           = flag.BoolP("add", "a", false, "添加或更新 token 信息到白名单")
	deleteB       = flag.BoolP("delete", "d", false, "删除白名单中的 token")
	generateToken = flag.BoolP("generate-token", "g", false, "生成 token 并输出")
	tokenLength   = flag.IntP("token-length", "l", 48, "指定生成 token 的长度")
	token         = flag.StringP("token", "t", "", "指定 token (长度在 [16,127] 之间，支持 UTF-8 字符)")
	message       = flag.StringP("message", "m", "", "备注 token 信息")
	service       = flag.StringP("service", "s", "", "指定需要采用的域名解析服务提供商，以下是可指定的提供商\n"+
		common.DNSPod+"\n"+
		common.AliDNS+"\n"+
		common.Cloudflare+"\n"+
		common.HuaweiCloud)
	domain = flag.StringP("domain", "D", "", "指定需要操作的域名")
	a      = flag.StringP("A", "A", "", "指定需要修改的 A 记录")
	aaaa   = flag.StringP("AAAA", "", "", "指定需要修改的 AAAA 记录 (默认同 A 记录，除非单独指定)")
)

func main() {
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
		server.ConfDir = filepath.Clean(*confDir)
	}

	if serviceAction != "" {
		svc, err := newService()
		if err != nil {
			log.Fatal(err)
		}
		intsvc.RunCommand(svc, serviceAction)
		return
	}

	if isRunMode {
		svc, err := newService()
		if err != nil {
			log.Fatal(err)
		}
		intsvc.Run(svc)
		return
	}

	exit, err := processFlag()
	if err != nil {
		log.Fatal(err)
	}
	if exit {
		return
	}

	startServer()
}

func newService() (*intsvc.Svc, error) {
	args := []string{"-c", absConfDir()}
	return intsvc.New(svcName, svcDisplayName, svcDescription, args, func() {
		if err := server.Srv.LoadConf(); err != nil {
			log.Fatal(err)
		}
		startServer()
	})
}

func absConfDir() string {
	dir := server.ConfDir
	if !filepath.IsAbs(dir) {
		if wd, err := os.Getwd(); err == nil {
			dir = filepath.Join(wd, dir)
		}
	}
	return dir
}

func processFlag() (exit bool, err error) {
	if *initOption != "" {
		for _, event := range *initOption {
			if err = initConf(string(event)); err != nil {
				return
			}
		}
		return true, nil
	}

	if *deleteB {
		var msg string
		if *token != "" {
			msg, err = server.DelFromWhitelist(*token)
		} else {
			err = errors.New("未指定 token")
		}
		if err != nil {
			return
		}
		fmt.Print(msg)
		return true, nil
	}

	var currentToken string
	switch {
	case *token != "":
		currentToken = *token
	case *generateToken:
		length := *tokenLength
		if length < 16 || length > 127 {
			err = errors.New("生成 token 的长度不符合要求")
			return
		}
		currentToken = server.GenerateToken(length)
		fmt.Println("Token: " + currentToken)
		exit = true
	}

	if *add {
		if len(*message) > 32 {
			err = errors.New("token message 备注信息过长")
			return
		}
		if currentToken == "" || len(currentToken) < 16 || len(currentToken) > 127 {
			err = errors.New("token 不符合要求")
			return
		}

		var status string
		status, err = server.AddToWhitelist(currentToken, *message, *service, *domain, *a, *aaaa)
		if err != nil {
			return
		}
		exit = true
		switch status {
		case server.InsertSign:
			fmt.Printf("Added %v(%v) to whitelist.\n", *message, currentToken)
		case server.UpdateSign:
			fmt.Printf("Updated %v(%v) in whitelist.\n", *message, currentToken)
		}
	}

	if exit {
		return
	}

	if err = server.Srv.LoadConf(); err != nil {
		return
	}

	if *version {
		server.Srv.CheckLatestVersion()
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
		msg, err = server.Srv.InitConf()
	case "1":
		msg, err = server.Services.InitConf()
	case "2":
		msg, err = server.InitWhitelist()
	default:
		err = errors.New("你初始化了一个寂寞")
	}
	if err != nil {
		return
	}
	log.Println(msg)
	return
}

func startServer() {
	if server.Srv.CenterService {
		if err := server.Services.LoadConf(); err != nil {
			log.Fatal(err)
		}
		http.HandleFunc(server.Srv.Route.Center, server.RespCenterReq)
	}

	http.HandleFunc(server.Srv.Route.GetIP, server.RespGetIPReq)

	httpSrv := http.Server{
		Addr:              server.Srv.ServerAddr,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       2 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	httpSrv.SetKeepAlivesEnabled(false)

	var err error
	if server.Srv.TLS.Enable {
		log.Println("Work on", server.Srv.ServerAddr, "with TLS")
		err = httpSrv.ListenAndServeTLS(server.Srv.TLS.CertFile, server.Srv.TLS.KeyFile)
	} else {
		log.Println("Work on", server.Srv.ServerAddr)
		err = httpSrv.ListenAndServe()
	}
	if err != nil {
		log.Fatal(err)
	}
}
