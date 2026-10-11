package server

import (
	"ddns-watchdog/internal/common"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// The echo endpoint is unauthenticated, so the upstream version lookup must not
// happen on every request.
func TestGetLatestVersionCachesUpstream(t *testing.T) {
	var hits atomic.Int32
	rootSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = fmt.Fprint(w, `{"latest_version":"9.9.9"}`)
	}))
	defer rootSrv.Close()

	origSrv, origCache, origCached := Srv, versionCache, versionCached
	t.Cleanup(func() {
		Srv, versionCache, versionCached = origSrv, origCache, origCached
	})

	Srv = server{RootServerUrl: rootSrv.URL}
	versionCache, versionCached = "", time.Time{}

	for range 3 {
		if got := Srv.GetLatestVersion(); got != "9.9.9" {
			t.Fatalf("GetLatestVersion = %q, want 9.9.9", got)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("upstream was queried %d times, want 1 (cached)", got)
	}

	// an expired cache must ask again
	versionCached = time.Now().Add(-2 * versionCacheTTL)
	if got := Srv.GetLatestVersion(); got != "9.9.9" {
		t.Fatalf("GetLatestVersion after expiry = %q, want 9.9.9", got)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("upstream was queried %d times, want 2 after the cache expired", got)
	}
}

// The root server answers from its own version and must never call out.
func TestGetLatestVersionRootServerSkipsUpstream(t *testing.T) {
	var hits atomic.Int32
	rootSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = fmt.Fprint(w, `{"latest_version":"9.9.9"}`)
	}))
	defer rootSrv.Close()

	origSrv, origCache, origCached := Srv, versionCache, versionCached
	t.Cleanup(func() {
		Srv, versionCache, versionCached = origSrv, origCache, origCached
	})

	Srv = server{IsRootServer: true, RootServerUrl: rootSrv.URL}
	versionCache, versionCached = "", time.Time{}

	if got := Srv.GetLatestVersion(); got != common.Version {
		t.Errorf("GetLatestVersion = %q, want %q", got, common.Version)
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("the root server was queried %d times, want 0", got)
	}
}
