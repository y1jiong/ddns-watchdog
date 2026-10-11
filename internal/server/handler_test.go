package server

import (
	"ddns-watchdog/internal/common"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// useTempConf points the package at an empty config directory with a clean
// server config and restores all package state afterward.
func useTempConf(t *testing.T) {
	t.Helper()

	origConfDir, origWhitelist := ConfDir, whitelist
	origModTime, origSize := whitelistModTime, whitelistSize
	origSrv := Srv

	ConfDir = t.TempDir()
	Srv = server{}

	whitelistMu.Lock()
	whitelist = nil
	whitelistModTime, whitelistSize = time.Time{}, 0
	whitelistMu.Unlock()

	t.Cleanup(func() {
		ConfDir = origConfDir
		Srv = origSrv
		whitelistMu.Lock()
		whitelist, whitelistModTime, whitelistSize = origWhitelist, origModTime, origSize
		whitelistMu.Unlock()
	})
}

func setWhitelist(t *testing.T, entries map[string]whitelistStruct) {
	t.Helper()

	whitelistMu.Lock()
	defer whitelistMu.Unlock()

	whitelist = entries
	whitelistModTime, whitelistSize = fileState(whitelistPath())
}

// Every rejection must carry a JSON body, otherwise a client can only report a
// bare status code.
func TestRespCenterReqRejectionsCarryJsonBody(t *testing.T) {
	useTempConf(t)
	setWhitelist(t, map[string]whitelistStruct{
		"disabled-token-0123456789": {Enable: false, Description: "off", Service: common.DNSPod},
	})

	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
	}{
		{
			name:       "wrong method",
			method:     http.MethodGet,
			body:       "",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "malformed body",
			method:     http.MethodPost,
			body:       "{",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown token",
			method:     http.MethodPost,
			body:       `{"token":"unknown-token-0123456789","enable":{"ipv4":true},"ip":{"ipv4":"1.2.3.4"}}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "disabled token",
			method:     http.MethodPost,
			body:       `{"token":"disabled-token-0123456789","enable":{"ipv4":true},"ip":{"ipv4":"1.2.3.4"}}`,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, "/center", strings.NewReader(tt.body))

			RespCenterReq(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var resp common.GeneralResp
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response %q is not JSON: %v", rec.Body.String(), err)
			}
			if resp.Message == "" {
				t.Errorf("response %q carries no message", rec.Body.String())
			}
		})
	}
}

func TestRespGetIPReqWrongMethodCarriesJsonBody(t *testing.T) {
	useTempConf(t)

	rec := httptest.NewRecorder()
	RespGetIPReq(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	var resp common.GeneralResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response %q is not JSON: %v", rec.Body.String(), err)
	}
	if resp.Message == "" {
		t.Errorf("response %q carries no message", rec.Body.String())
	}
}

func TestRespGetIPReqReturnsAddressAsJson(t *testing.T) {
	useTempConf(t)
	Srv = server{IsRootServer: true} // the root server answers without an upstream lookup

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:1234"

	RespGetIPReq(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}

	var resp common.GetIPResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response %q is not JSON: %v", rec.Body.String(), err)
	}
	if resp.IP != "203.0.113.9" {
		t.Errorf("ip = %q, want 203.0.113.9", resp.IP)
	}
	if resp.Version != common.Version {
		t.Errorf("latest_version = %q, want %q", resp.Version, common.Version)
	}
}

// A provider that is not configured on this server must say so.
func TestDoVirtualClientDisabledProvider(t *testing.T) {
	origServices := Services
	t.Cleanup(func() { Services = origServices })
	Services = service{}

	status, resp := doVirtualClient(common.CenterReq{}, whitelistStruct{Service: common.DNSPod})

	if status != http.StatusForbidden {
		t.Errorf("status = %d, want %d", status, http.StatusForbidden)
	}
	if !strings.Contains(resp.Message, common.DNSPod) {
		t.Errorf("message %q does not name the provider", resp.Message)
	}
}

func TestDoVirtualClientUnsupportedProvider(t *testing.T) {
	status, resp := doVirtualClient(common.CenterReq{}, whitelistStruct{Service: "route53"})

	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", status, http.StatusBadRequest)
	}
	if !strings.Contains(resp.Message, "route53") {
		t.Errorf("message %q does not name the provider", resp.Message)
	}
}

// A provider failure must not look like success: the client uses the status to
// decide whether to retry on its next cycle.
func TestCenterRespStatusForProviderErrors(t *testing.T) {
	tests := []struct {
		name       string
		msg        []string
		errs       []error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "no changes needed",
			wantStatus: http.StatusOK,
		},
		{
			name:       "updated",
			msg:        []string{"dnspod: home.example.com updated"},
			wantStatus: http.StatusOK,
			wantBody:   "updated",
		},
		{
			name:       "provider failed",
			errs:       []error{errString("record not found")},
			wantStatus: http.StatusBadGateway,
			wantBody:   "record not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, resp := centerResp(tt.msg, tt.errs)

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(resp.Message, tt.wantBody) {
				t.Errorf("message %q does not contain %q", resp.Message, tt.wantBody)
			}
		})
	}
}

type errString string

func (e errString) Error() string { return string(e) }
