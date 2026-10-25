package server

import (
	"ddns-watchdog/internal/common"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

	origSrv, origCache, origCacheErr, origCached := Srv, versionCache, versionCacheErr, versionCached
	t.Cleanup(func() {
		Srv, versionCache, versionCacheErr, versionCached = origSrv, origCache, origCacheErr, origCached
	})

	Srv = server{RootServerUrl: rootSrv.URL}
	versionCache, versionCacheErr, versionCached = "", nil, time.Time{}

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

	origSrv, origCache, origCacheErr, origCached := Srv, versionCache, versionCacheErr, versionCached
	t.Cleanup(func() {
		Srv, versionCache, versionCacheErr, versionCached = origSrv, origCache, origCacheErr, origCached
	})

	Srv = server{IsRootServer: true, RootServerUrl: rootSrv.URL}
	versionCache, versionCacheErr, versionCached = "", nil, time.Time{}

	if got := Srv.GetLatestVersion(); got != common.Version {
		t.Errorf("GetLatestVersion = %q, want %q", got, common.Version)
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("the root server was queried %d times, want 0", got)
	}
}

// An unreachable upstream must not be retried on every request either, and the
// cached error must still carry the concrete reason for -V.
func TestGetLatestVersionCachesFailure(t *testing.T) {
	var hits atomic.Int32
	rootSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "maintenance", http.StatusServiceUnavailable)
	}))
	defer rootSrv.Close()

	origSrv, origCache, origCacheErr, origCached := Srv, versionCache, versionCacheErr, versionCached
	t.Cleanup(func() {
		Srv, versionCache, versionCacheErr, versionCached = origSrv, origCache, origCacheErr, origCached
	})

	Srv = server{RootServerUrl: rootSrv.URL}
	versionCache, versionCacheErr, versionCached = "", nil, time.Time{}

	for range 2 {
		if got := Srv.GetLatestVersion(); !strings.Contains(got, "N/A") {
			t.Fatalf("GetLatestVersion = %q, want N/A for an unavailable upstream", got)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("upstream was queried %d times, want 1 (failure cached)", got)
	}

	_, err := Srv.latestVersion()
	if err == nil {
		t.Fatal("latestVersion returned no error for an unavailable upstream")
	}
	if !strings.Contains(err.Error(), "HTTP 503") {
		t.Errorf("error %q does not name the HTTP status", err)
	}
}
