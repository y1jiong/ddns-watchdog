package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// versionFetchTimeout bounds the update check made by -V. A version lookup is
// best-effort, so it must not keep the command waiting for the general client
// timeout when the network is unreachable.
const versionFetchTimeout = 10 * time.Second

type versionResult struct {
	version string
	err     error
}

// LatestVersion asks apiURL for the latest released version. userAgent
// identifies the calling binary, which the version service uses to reject
// clients that are too old. The error names the concrete failure (timeout, DNS,
// HTTP status, bad JSON) so -V can report why the check failed.
func LatestVersion(apiURL, userAgent string) (string, error) {
	return fetchLatestVersion(apiURL, userAgent, versionFetchTimeout)
}

func fetchLatestVersion(apiURL, userAgent string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid version API URL %q: %w", apiURL, err)
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}

	resp, err := DefaultHttpClient.Do(req)
	if err != nil {
		return "", describeVersionError(err, timeout)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("version API returned HTTP %s", resp.Status)
	}

	// cap the body so a misbehaving endpoint cannot make the check read forever
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("reading version API response: %w", err)
	}

	var res GetIPResp
	if err = json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("invalid JSON response from version API: %w", err)
	}
	if res.Version == "" {
		return "", errors.New(`version API response has no "latest_version" field`)
	}

	return res.Version, nil
}

// describeVersionError turns a transport error into a short, specific reason
// while keeping the original detail (dial error, TLS error, DNS name) attached.
func describeVersionError(err error, timeout time.Duration) error {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Errorf("request timed out after %s", timeout)
	}
	if errors.Is(err, context.Canceled) {
		return errors.New("request canceled")
	}
	return fmt.Errorf("request failed: %w", err)
}

// VersionTips prints the local build information and the latest version. The
// lookup runs in the background, so the local information is printed while the
// request is in flight. fetch is nil when there is nothing to ask, as on the root
// server.
func VersionTips(apiURL string, fetch func() (string, error)) {
	printVersionTips(os.Stdout, apiURL, fetch)
}

func printVersionTips(w io.Writer, apiURL string, fetch func() (string, error)) {
	var result <-chan versionResult
	if fetch != nil {
		ch := make(chan versionResult, 1)
		result = ch
		go func() {
			version, err := fetch()
			ch <- versionResult{version: version, err: err}
		}()
	}

	_, _ = fmt.Fprintln(w, "current version:", Version)
	_, _ = fmt.Fprintln(w, "git commit:", GitCommit)
	_, _ = fmt.Fprintln(w, "build time:", BuildTime)

	if fetch == nil {
		return
	}

	// The default endpoint is not worth echoing; show where only a custom API is
	// used.
	if apiURL == "" || apiURL == DefaultAPIUrl {
		_, _ = fmt.Fprintln(w, "checking latest version ...")
	} else {
		_, _ = fmt.Fprintf(w, "checking latest version at %s ...\n", apiURL)
	}

	res := <-result
	// A non-root server answers with "N/A (...)" when its own upstream is
	// unavailable; that is a failed lookup too, not a version number.
	if res.err == nil && strings.HasPrefix(res.version, "N/A") {
		res.err = errors.New(res.version)
	}
	if res.err != nil {
		_, _ = fmt.Fprintln(w, "latest version: unavailable")
		_, _ = fmt.Fprintln(w, "error:", res.err)
		_, _ = fmt.Fprintln(w, "\ncheck for updates manually at "+projectUrl)
		return
	}

	_, _ = fmt.Fprintln(w, "latest version:", res.version)
	if semver.Compare("v"+Version, "v"+res.version) < 0 {
		_, _ = fmt.Fprintln(w, "\nnew version available, visit "+projectUrl+" to download")
	}
}
