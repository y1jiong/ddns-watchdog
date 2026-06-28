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
	confDir         = flag.StringP("conf", "c", "", "config directory (use quotes if path contains spaces)")
	installOption   = flag.BoolP("install", "I", false, "install service and exit")
	uninstallOption = flag.BoolP("uninstall", "U", false, "uninstall service and exit")
	version         = flag.BoolP("version", "V", false, "print version and check for updates, then exit")
	initOption      = flag.StringP("init", "i", "", "selectively initialize config files and exit; can be combined (e.g. 01)\n"+
		"0 -> "+server.ConfFilename+"\n"+
		"1 -> "+server.ServiceConfFilename+"\n"+
		"2 -> "+server.WhitelistFilename)
	add           = flag.BoolP("add", "a", false, "add or update a token in the whitelist")
	deleteB       = flag.BoolP("delete", "d", false, "delete a token from the whitelist")
	generateToken = flag.BoolP("generate-token", "g", false, "generate a token and print it")
	tokenLength   = flag.IntP("token-length", "l", 48, "length of the generated token")
	token         = flag.StringP("token", "t", "", "token to use (16-127 chars, supports UTF-8)")
	message       = flag.StringP("message", "m", "", "description for the token")
	service       = flag.StringP("service", "s", "", "DNS provider to use, available options:\n"+
		common.DNSPod+"\n"+
		common.AliDNS+"\n"+
		common.Cloudflare+"\n"+
		common.HuaweiCloud)
	domain = flag.StringP("domain", "D", "", "domain to update")
	a      = flag.StringP("A", "A", "", "A record subdomain to update")
	aaaa   = flag.StringP("AAAA", "", "", "AAAA record subdomain (defaults to the same as -A unless specified)")
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

// absConfDir returns an absolute path for the conf directory.
// The service unit file records this path; a relative path would break when systemd/launchd
// starts the daemon from a different working directory.
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
			err = errors.New("no token specified")
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
			err = errors.New("token length must be between 16 and 127")
			return
		}
		currentToken = server.GenerateToken(length)
		fmt.Println("Token: " + currentToken)
		exit = true
	}

	if *add {
		if len(*message) > 32 {
			err = errors.New("token message too long (max 32 chars)")
			return
		}
		if currentToken == "" || len(currentToken) < 16 || len(currentToken) > 127 {
			err = errors.New("token must be 16-127 characters")
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
		err = errors.New("unknown init option: " + event)
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
