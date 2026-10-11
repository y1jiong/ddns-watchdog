package main

import (
	"context"
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
	confDir         = flag.StringP("conf", "c", "", "config directory (use quotes if path contains spaces)")
	installOption   = flag.BoolP("install", "I", false, "install service and exit")
	uninstallOption = flag.BoolP("uninstall", "U", false, "uninstall service and exit")
	enforcement     = flag.BoolP("force", "f", false, "force DNS record update regardless of IP change")
	version         = flag.BoolP("version", "V", false, "print version and check for updates, then exit")
	initOption      = flag.StringP("init", "i", "", "selectively initialize config files and exit; can be combined (e.g. 01)\n"+
		"0 -> "+client.ConfFilename+"\n"+
		"1 -> "+client.DNSPodConfFilename+"\n"+
		"2 -> "+client.AliDNSConfFilename+"\n"+
		"3 -> "+client.CloudflareConfFilename+"\n"+
		"4 -> "+client.HuaweiCloudConfFilename)
	printNetworkCardInfo = flag.BoolP("network-card", "n", false, "print network card info and exit")
)

func main() {
	flag.Usage = func() {
		intsvc.PrintUsage(os.Stderr, filepath.Base(os.Args[0]), flag.PrintDefaults)
	}
	flag.Parse()

	// Positional subcommands: "run" (invoked by the OS service manager) and
	// "service <action>". An unknown or incomplete one is an error.
	sub, err := intsvc.ParseSubcommand(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}

	if *confDir != "" {
		client.ConfDir = filepath.Clean(*confDir)
	}

	// "service <action>" subcommand
	if sub.Action != "" {
		if sub.Action == "install" {
			if err = client.Client.LoadConf(); err != nil {
				log.Fatal(err)
			}
			if err = installService(); err != nil {
				log.Fatal(err)
			}
			return
		}
		svc, err := newService()
		if err != nil {
			log.Fatal(err)
		}
		intsvc.RunCommand(svc, sub.Action)
		return
	}

	// "run" positional arg: invoked by the OS service manager
	if sub.RunMode {
		svc, err := newService()
		if err != nil {
			log.Fatal(err)
		}
		intsvc.Run(svc)
		return
	}

	// Standard flag-based flow
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

	runLoop(context.Background())
}

func newService() (*intsvc.Svc, error) {
	args := []string{"-c", absConfDir()}
	return intsvc.New(svcName, svcDisplayName, svcDescription, args, startDaemon)
}

// installService installs the OS service. The caller must have loaded the client
// config already.
// A service whose check cycle is <= 0 would run a single check and exit, which is
// never what "install a service" should mean, so that is refused.
func installService() error {
	if client.Client.CheckCycleMinutes <= 0 {
		return errors.New("check_cycle_minutes in " + client.ConfDir + "/" + client.ConfFilename +
			" must be greater than 0 to install the service")
	}

	svc, err := newService()
	if err != nil {
		return err
	}
	intsvc.RunCommand(svc, "install")
	return nil
}

func startDaemon(ctx context.Context) {
	if err := client.Client.LoadConf(); err != nil {
		log.Fatal(err)
	}
	if err := loadConf(); err != nil {
		log.Fatal(err)
	}
	runLoop(ctx)
}

// absConfDir returns an absolute path for the conf directory.
// The service unit file records this path; a relative path would break when systemd/launchd
// starts the daemon from a different working directory.
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
		return true, installService()
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
		err = errors.New("unknown init option: " + event)
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

func runLoop(ctx context.Context) {
	if client.Client.CheckCycleMinutes <= 0 {
		check()
		return
	}

	cycle := time.NewTicker(time.Duration(client.Client.CheckCycleMinutes) * time.Minute)
	defer cycle.Stop()

	for {
		check()
		select {
		case <-cycle.C:
		case <-ctx.Done():
			return
		}
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

	// Only remember the addresses once the update has been delivered. Committing
	// them first would make the next cycle consider the records up to date and
	// never retry a transient failure.
	updated := false
	if client.Client.Center.Enable {
		updated = updateCenter(ipv4, ipv6)
	} else {
		updated = updateRecords(ipv4, ipv6)
	}
	if !updated {
		return
	}

	client.Client.LatestIPv4 = ipv4
	client.Client.LatestIPv6 = ipv6
}

// updateCenter reports the addresses to the center proxy.
func updateCenter(ipv4, ipv6 string) (ok bool) {
	msg, err := client.AccessCenter(ipv4, ipv6)
	for _, row := range msg {
		log.Println(row)
	}
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

// updateRecords runs every enabled provider and reports whether they all
// succeeded.
func updateRecords(ipv4, ipv6 string) (ok bool) {
	var (
		wg      sync.WaitGroup
		results = make(chan bool, 4)
	)

	run := func(callback client.ServiceCallback) {
		wg.Go(func() { results <- serviceInterface(ipv4, ipv6, callback) })
	}
	if client.Client.Services.DNSPod {
		run(client.DP.Run)
	}
	if client.Client.Services.AliDNS {
		run(client.AD.Run)
	}
	if client.Client.Services.Cloudflare {
		run(client.Cf.Run)
	}
	if client.Client.Services.HuaweiCloud {
		run(client.HC.Run)
	}

	wg.Wait()
	close(results)

	ok = true
	for success := range results {
		if !success {
			ok = false
		}
	}
	return
}

// serviceInterface runs one provider and reports whether it updated everything
// that was asked of it.
func serviceInterface(ipv4, ipv6 string, callback client.ServiceCallback) bool {
	msg, errs := callback(client.Client.Enable, ipv4, ipv6)
	for _, row := range errs {
		log.Println(row)
	}
	for _, row := range msg {
		log.Println(row)
	}
	return len(errs) == 0
}
