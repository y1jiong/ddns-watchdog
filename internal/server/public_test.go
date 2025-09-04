package server

import (
	"net/http"
	"net/textproto"
	"testing"
)

func TestGetClientIP(t *testing.T) {
	cases := []struct {
		name   string
		req    *http.Request
		expect string
	}{
		{
			name:   "X-Real-IP IPv4",
			req:    &http.Request{Header: http.Header{textproto.CanonicalMIMEHeaderKey("X-Real-IP"): []string{"192.168.1.1"}}},
			expect: "192.168.1.1",
		},
		{
			name:   "X-Real-IP IPv6",
			req:    &http.Request{Header: http.Header{textproto.CanonicalMIMEHeaderKey("X-Real-IP"): []string{"2001:db8::1"}}},
			expect: "2001:db8:0:0:0:0:0:1",
		},
		{
			name:   "X-Real-IP IPv6 Short",
			req:    &http.Request{Header: http.Header{textproto.CanonicalMIMEHeaderKey("X-Real-IP"): []string{"::1"}}},
			expect: "0:0:0:0:0:0:0:1",
		},
		{
			name:   "X-Forwarded-For IPv4",
			req:    &http.Request{Header: http.Header{textproto.CanonicalMIMEHeaderKey("X-Forwarded-For"): []string{"203.0.113.1, 192.168.1.1"}}},
			expect: "203.0.113.1",
		},
		{
			name:   "X-Forwarded-For IPv6",
			req:    &http.Request{Header: http.Header{textproto.CanonicalMIMEHeaderKey("X-Forwarded-For"): []string{"2001:db8::1, 192.168.1.1"}}},
			expect: "2001:db8:0:0:0:0:0:1",
		},
		{
			name:   "X-Forwarded-For IPv6 Short",
			req:    &http.Request{Header: http.Header{textproto.CanonicalMIMEHeaderKey("X-Forwarded-For"): []string{"::1, 192.168.1.1"}}},
			expect: "0:0:0:0:0:0:0:1",
		},
		{
			name:   "RemoteAddr IPv4",
			req:    &http.Request{RemoteAddr: "192.168.1.1:8080"},
			expect: "192.168.1.1",
		},
		{
			name:   "RemoteAddr IPv6",
			req:    &http.Request{RemoteAddr: "[2001:db8::1]:8080"},
			expect: "2001:db8:0:0:0:0:0:1",
		},
		{
			name:   "RemoteAddr IPv6 Short",
			req:    &http.Request{RemoteAddr: "[::1]:8080"},
			expect: "0:0:0:0:0:0:0:1",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ip := GetClientIP(c.req)
			if ip != c.expect {
				t.Errorf("%s: expected %s, got %s", c.name, c.expect, ip)
			}
		})
	}
}
