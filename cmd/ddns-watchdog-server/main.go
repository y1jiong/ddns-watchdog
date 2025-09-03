package main

import (
	"crypto/tls"
	"ddns-watchdog/internal/common"
	"ddns-watchdog/internal/server"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	flag "github.com/spf13/pflag"
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
	// 处理 flag
	exit, err := processFlag()
	if err != nil {
		log.Fatal(err)
	}
	if exit {
		return
	}

	// 加载白名单
	if server.Srv.CenterService {
		if err = server.Services.LoadConf(); err != nil {
			log.Fatal(err)
		}
		// 路由绑定函数
		http.HandleFunc(server.Srv.Route.Center, server.RespCenterReq)
	}

	// 路由绑定函数
	http.HandleFunc(server.Srv.Route.GetIP, server.RespGetIPReq)

	// 设置超时参数和最低 TLS 版本
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

	// 启动监听
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

func processFlag() (exit bool, err error) {
	flag.Parse()

	if *confDir != "" {
		server.ConfDir = filepath.Clean(*confDir)
	}

	// 初始化配置
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
	// 获取 token
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

	// 添加 token 到白名单
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

	// 若无必要，不加载配置
	if exit {
		return
	}

	// 加载配置
	if err = server.Srv.LoadConf(); err != nil {
		return
	}

	// 版本信息
	if *version {
		server.Srv.CheckLatestVersion()
		return true, nil
	}

	// 安装 / 卸载服务
	switch {
	case *installOption:
		return true, server.Install()
	case *uninstallOption:
		return true, server.Uninstall()
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
