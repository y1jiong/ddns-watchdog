package client

import (
	"ddns-watchdog/internal/common"
	"io"
	"net/http"
)

const projName = "ddns-watchdog-client"

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

// mapString reads a string field from a decoded JSON object. ok is false when the
// field is missing or holds another type, which is what happens if a provider
// changes the shape of its response. Callers skip or report such entries instead
// of panicking on a failed type assertion.
func mapString(m map[string]any, key string) (s string, ok bool) {
	s, ok = m[key].(string)
	return
}
