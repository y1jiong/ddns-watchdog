package client

import (
	"ddns-watchdog/internal/common"
	"net/http"
)

const projName = "ddns-watchdog-client"

var installPath = "/etc/systemd/system/" + projName + ".service"

func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", projName+"/"+common.Version)
	return common.DefaultHttpClient.Do(req)
}
