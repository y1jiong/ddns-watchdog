package client

import (
	"ddns-watchdog/internal/common"
	"io"
	"net/http"
)

const (
	projName    = "ddns-watchdog-client"
	installPath = "/etc/systemd/system/" + projName + ".service"
)

func httpGet(url string) (*http.Response, error) {
	req, err := httpNewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return common.DefaultHttpClient.Do(req)
}

func httpNewRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", projName+"/"+common.Version)
	return req, nil
}
