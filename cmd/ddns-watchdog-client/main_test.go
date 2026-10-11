package main

import (
	"ddns-watchdog/internal/client"
	"ddns-watchdog/internal/common"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// A failed update must not be remembered as done: otherwise the next cycle sees
// "no change" and the record stays stale until the address changes again.
func TestCheckRetriesAfterFailedUpdate(t *testing.T) {
	ipSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ip":"203.0.113.7"}`))
	}))
	defer ipSrv.Close()

	var failing atomic.Bool
	failing.Store(true)
	centerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"invalid token"}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":"updated"}`))
	}))
	defer centerSrv.Close()

	origClient := client.Client
	t.Cleanup(func() { client.Client = origClient })

	client.Client.Enable = common.Enable{IPv4: true}
	client.Client.APIUrl.IPv4 = ipSrv.URL
	client.Client.Center.Enable = true
	client.Client.Center.APIUrl = centerSrv.URL
	client.Client.Center.Token = "0123456789abcdef"
	client.Client.LatestIPv4 = ""
	client.Client.LatestIPv6 = ""

	check()
	if got := client.Client.LatestIPv4; got != "" {
		t.Fatalf("LatestIPv4 = %q after a rejected update, want it unset so the next cycle retries", got)
	}

	failing.Store(false)
	check()
	if got := client.Client.LatestIPv4; got != "203.0.113.7" {
		t.Fatalf("LatestIPv4 = %q after a successful update, want 203.0.113.7", got)
	}

	// once the address is known, an unchanged address must not trigger more work
	client.Client.Center.Enable = false
	check()
	if got := client.Client.LatestIPv4; got != "203.0.113.7" {
		t.Fatalf("LatestIPv4 = %q, want it left alone when the address did not change", got)
	}
}
