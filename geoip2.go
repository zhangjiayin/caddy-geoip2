// Package geoip2 provides middleware for resolving a user's IP address
// using the MaxMind GeoIP2 database.
package geoip2

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/zhangjiayin/caddy-geoip2/replacer"
	"go.uber.org/zap"
)

// GeoIP2 implements the http.handlers.geoip2 middleware.
// It uses GeoIP2 Data to determine the geographic location
// of a client's IP address.
type GeoIP2 struct {
	Enable string `json:"enable,omitempty"`

	state *GeoIP2State
	ctx   caddy.Context
	mode  mode
}

type mode string

// These are the possible values GeoIP2.Enable can have:
// - "strict" only uses remote IP address.
// - "wild" uses X-Forwarded-For if it exists.
// - "trusted_proxies" uses X-Forwarded-For if it exists when trusted_proxies is valid,
// see https://caddyserver.com/docs/caddyfile/options#trusted-proxies.
// - Lookups can also be disabled by setting it to "off", "false" or "0".
//
// The handler defaults to TrustedProxies if the variable is not set.
const (
	modeStrict         mode = "strict"
	modeTrustedProxies mode = "trusted_proxies"
	modeWild           mode = "wild"
	modeDisabled       mode = "disabled"
)

func init() {
	caddy.RegisterModule(&GeoIP2{})
	httpcaddyfile.RegisterHandlerDirective("geoip2_vars", parseCaddyfile)
}

// CaddyModule implements caddy.Module.
func (m *GeoIP2) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.geoip2",
		New: func() caddy.Module { return m },
	}
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (m *GeoIP2) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	replacer.SetDefaultValues(repl)

	if m.mode != modeDisabled {
		if m.state != nil && m.state.dbReaders != nil {
			clientIP, err := m.getClientIP(r)
			if err != nil {
				caddy.Log().Named("http.handlers.geoip2").Error(
					"getting client IP address",
					zap.Error(err),
				)
			} else {
				if len(clientIP) > 0 {
					repl.Set("geoip2.ip_address", clientIP.String())
				}
				m.state.lookup(repl, clientIP)
			}
		}
	}
	return next.ServeHTTP(w, r)
}

func (m *GeoIP2) getClientIP(r *http.Request) (net.IP, error) {
	var ip string
	trustedProxy := caddyhttp.GetVar(r.Context(), caddyhttp.TrustedProxyVarKey).(bool)
	fwdFor := r.Header.Get("X-Forwarded-For")

	if ((m.mode == modeTrustedProxies && trustedProxy) || m.mode == modeWild) && fwdFor != "" {
		ips := strings.Split(fwdFor, ", ")
		ip = ips[0]
	} else {
		// Otherwise, get the client ip from the request remote address.
		var err error
		ip, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			var addrErr *net.AddrError
			if errors.As(err, &addrErr) && addrErr.Err == "missing port in address" {
				// It's not critical, attempt to use RemoteAddr as-is
				ip = r.RemoteAddr
			} else {
				return nil, err
			}
		}
	}

	// Parse the ip address string into a net.IP.
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, fmt.Errorf("unable to parse address: %q", ip)
	}
	return parsedIP, nil
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	m := &GeoIP2{}
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (m *GeoIP2) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if !d.Args(&m.Enable) {
			return d.ArgErr()
		}
	}
	return nil
}

// Provision implements caddy.Provisioner.
func (m *GeoIP2) Provision(ctx caddy.Context) error {
	caddy.Log().Named("http.handlers.geoip2").Debug("provision")
	app, err := ctx.App(moduleName)
	if err != nil {
		return fmt.Errorf("getting geoip2 app: %w", err)
	}
	m.state = app.(*GeoIP2State)
	m.ctx = ctx

	switch strings.ToLower(m.Enable) {
	case string(modeStrict):
		m.mode = modeStrict
	case string(modeWild):
		m.mode = modeWild
	case "off", "false", "0":
		m.mode = modeDisabled
	default:
		m.mode = modeTrustedProxies
	}
	return nil
}

// Validate implements caddy.Validator.
func (m *GeoIP2) Validate() error {
	caddy.Log().Named("http.handlers.geoip2").Debug("validate")
	return nil
}

// Interface guards.
var (
	_ caddy.Module                = (*GeoIP2)(nil)
	_ caddy.Provisioner           = (*GeoIP2)(nil)
	_ caddy.Validator             = (*GeoIP2)(nil)
	_ caddyhttp.MiddlewareHandler = (*GeoIP2)(nil)
	_ caddyfile.Unmarshaler       = (*GeoIP2)(nil)
)
