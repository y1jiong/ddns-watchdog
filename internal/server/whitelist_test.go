package server

import (
	"ddns-watchdog/internal/common"
	"os"
	"sync"
	"testing"
)

// A token deleted by "ddns-watchdog-server -d" while the server is running must
// stop working without a restart.
func TestGetWhitelistEntryNoticesRevokedToken(t *testing.T) {
	useTempConf(t)

	if _, err := InitWhitelist(); err != nil {
		t.Fatalf("InitWhitelist: %v", err)
	}

	const token = "0123456789abcdef"
	if _, err := AddToWhitelist(token, "home", common.DNSPod, "example.com", "@", ""); err != nil {
		t.Fatalf("AddToWhitelist: %v", err)
	}
	if _, ok := getWhitelistEntry(token); !ok {
		t.Fatal("token is not accepted right after being added")
	}

	// simulate the edit another process would make
	if err := os.WriteFile(whitelistPath(), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, ok := getWhitelistEntry(token); ok {
		t.Error("revoked token is still accepted; the whitelist was not reloaded")
	}
}

// A token added by the CLI while the server is running must start working, and
// entries that are gone from the file must not survive in memory.
func TestGetWhitelistEntryNoticesAddedToken(t *testing.T) {
	useTempConf(t)

	if _, err := InitWhitelist(); err != nil {
		t.Fatalf("InitWhitelist: %v", err)
	}

	const (
		oldToken = "0123456789abcdef"
		newToken = "fedcba9876543210"
	)
	if _, err := AddToWhitelist(oldToken, "home", common.DNSPod, "example.com", "@", ""); err != nil {
		t.Fatalf("AddToWhitelist: %v", err)
	}

	replacement := `{"` + newToken + `":{"enable":true,"description":"lab",` +
		`"service":"dnspod","domain_record":{"domain":"example.com",` +
		`"subdomain":{"a":"@","aaaa":"@"}}}}`
	if err := os.WriteFile(whitelistPath(), []byte(replacement), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, ok := getWhitelistEntry(newToken); !ok {
		t.Error("token added to the file is not accepted; the whitelist was not reloaded")
	}
	if _, ok := getWhitelistEntry(oldToken); ok {
		t.Error("token removed from the file is still accepted; the reload merged instead of replacing")
	}
}

// Lookups happen on every request while the file may be replaced underneath, so
// the reload must be safe under concurrency (run with -race).
func TestGetWhitelistEntryConcurrentReload(t *testing.T) {
	useTempConf(t)

	if _, err := InitWhitelist(); err != nil {
		t.Fatalf("InitWhitelist: %v", err)
	}

	const token = "0123456789abcdef"
	withToken := `{"` + token + `":{"enable":true,"description":"home",` +
		`"service":"dnspod","domain_record":{"domain":"example.com",` +
		`"subdomain":{"a":"@","aaaa":"@"}}}}`

	write := func(content string) {
		t.Helper()
		// replace atomically, the way a reader expects to find the file
		tmp := whitelistPath() + ".tmp"
		if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(tmp, whitelistPath()); err != nil {
			t.Fatal(err)
		}
	}
	write("{}")

	var (
		wg   sync.WaitGroup
		stop = make(chan struct{})
	)
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = getWhitelistEntry(token)
				}
			}
		})
	}

	for i := range 20 {
		if i%2 == 0 {
			write(withToken)
		} else {
			write("{}")
		}
	}

	close(stop)
	wg.Wait()
}
