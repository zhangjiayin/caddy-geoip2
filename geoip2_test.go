package geoip2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

// requestWith builds a request the way a handler sees it: Caddy has put the
// trusted proxy flag and the client IP into the request's variables.
func requestWith(remoteAddr, forwardedFor string, vars map[string]any) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		r.Header.Set("X-Forwarded-For", forwardedFor)
	}
	if vars != nil {
		r = r.WithContext(context.WithValue(r.Context(), caddyhttp.VarsCtxKey, vars))
	}
	return r
}

func TestGetClientIP(t *testing.T) {
	const (
		client = "198.51.100.7"
		forged = "8.8.8.8"
		cdn    = "173.245.48.10"
	)
	// A client behind a CDN sends its own X-Forwarded-For entry; the CDN
	// appends the address it saw. Caddy, with trusted_proxies_strict, reads
	// from the right and has determined the client.
	trusted := map[string]any{
		caddyhttp.TrustedProxyVarKey: true,
		caddyhttp.ClientIPVarKey:     client,
	}
	// A direct connection that sends a forged header.
	untrusted := map[string]any{
		caddyhttp.TrustedProxyVarKey: false,
		caddyhttp.ClientIPVarKey:     client,
	}

	tests := []struct {
		name         string
		mode         string
		remoteAddr   string
		forwardedFor string
		vars         map[string]any
		want         string
	}{
		{"client_ip behind a trusted proxy takes Caddy's client IP", "client_ip", cdn + ":443", forged + ", " + client, trusted, client},
		{"client_ip ignores the header of an untrusted peer", "client_ip", client + ":50000", forged, untrusted, client},
		{"client_ip is case insensitive", "Client_IP", cdn + ":443", forged + ", " + client, trusted, client},
		{"client_ip falls back to the connection when Caddy has none", "client_ip", client + ":50000", forged, map[string]any{caddyhttp.ClientIPVarKey: ""}, client},
		{"client_ip falls back to the connection without variables", "client_ip", client + ":50000", forged, nil, client},
		{"client_ip with an IPv6 client", "client_ip", cdn + ":443", "2001:db8::1", map[string]any{caddyhttp.TrustedProxyVarKey: true, caddyhttp.ClientIPVarKey: "2001:db8::1"}, "2001:db8::1"},

		{"strict takes the connection", "strict", cdn + ":443", forged + ", " + client, trusted, cdn},
		{"trusted_proxies takes the first entry from a trusted proxy", "trusted_proxies", cdn + ":443", forged + ", " + client, trusted, forged},
		{"trusted_proxies takes the connection of an untrusted peer", "trusted_proxies", client + ":50000", forged, untrusted, client},
		{"the default is trusted_proxies", "", cdn + ":443", forged + ", " + client, trusted, forged},
		{"wild takes the first entry from anyone", "wild", client + ":50000", forged, untrusted, forged},
		{"wild without a header takes the connection", "wild", client + ":50000", "", untrusted, client},
		{"a connection address without a port", "strict", client, "", untrusted, client},
		{"an IPv6 connection", "strict", "[2001:db8::2]:50000", "", untrusted, "2001:db8::2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := GeoIP2{Enable: tt.mode}
			got, err := m.getClientIP(requestWith(tt.remoteAddr, tt.forwardedFor, tt.vars))
			if err != nil {
				t.Fatalf("getClientIP: %v", err)
			}
			if got.String() != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGetClientIPRejectsWhatIsNoAddress(t *testing.T) {
	m := GeoIP2{Enable: "wild"}
	if _, err := m.getClientIP(requestWith("198.51.100.7:50000", "not-an-ip", nil)); err == nil {
		t.Error("expected an error for a header that is no IP address")
	}
}

func TestValidateModes(t *testing.T) {
	for _, mode := range []string{"strict", "wild", "trusted_proxies", "client_ip", "CLIENT_IP", "off", "false", "0", ""} {
		if err := (GeoIP2{Enable: mode}).Validate(); err != nil {
			t.Errorf("mode %q should be valid: %v", mode, err)
		}
	}
	for _, mode := range []string{"clientip", "client-ip", "on"} {
		if err := (GeoIP2{Enable: mode}).Validate(); err == nil {
			t.Errorf("mode %q should be rejected", mode)
		}
	}
}
