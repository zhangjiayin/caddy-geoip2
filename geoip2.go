package geoip2

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type GeoIP2Record struct {
	Country struct {
		ISOCode           string            `maxminddb:"iso_code"`
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
		Names             map[string]string `maxminddb:"names"`
		GeoNameID         uint64            `maxminddb:"geoname_id"`
	} `maxminddb:"country"`

	City struct {
		Names     map[string]string `maxminddb:"names"`
		GeoNameID uint64            `maxminddb:"geoname_id"`
	} `maxminddb:"city"`

	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
		TimeZone  string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
}

type GeoIP2 struct {
	Enable string `json:"enable,omitempty"`
}

func init() {
	caddy.RegisterModule(GeoIP2{})
	httpcaddyfile.RegisterHandlerDirective("geoip2_vars", parseCaddyfile)
	httpcaddyfile.RegisterGlobalOption("geoip2", parseGeoip2)
	caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("init"))
}

func (GeoIP2) CaddyModule() caddy.ModuleInfo {
	caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("CaddyModule"))

	return caddy.ModuleInfo{
		ID:  "http.handlers.geoip2",
		New: func() caddy.Module { return new(GeoIP2) },
	}
}

func (m GeoIP2) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if m.Enable != "off" && m.Enable != "false" && m.Enable != "0" {
		var record = GeoIP2Record{}
		if geoIP2State.DBHandler != nil {
			strict := m.Enable == "strict"
			clientIP, _ := getClientIP(r, strict)
			geoIP2State.DBHandler.Lookup(clientIP, &record)
			repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set("geoip2.country_code", record.Country.ISOCode)
			val, ok := record.Country.Names["en"]
			if ok {
				repl.Set("geoip2.country_name", val)
			}
			repl.Set("geoip2.country_eu", record.Country.IsInEuropeanUnion)

			val, ok = record.City.Names["en"]
			if ok {
				repl.Set("geoip2.city_name", val)
			}
			repl.Set("geoip2.latitude", record.Location.Latitude)
			repl.Set("geoip2.longitude", record.Location.Longitude)
			repl.Set("geoip2.time_zone", record.Location.TimeZone)
			caddy.Log().Named("http.handlers.geoip2").Debug(fmt.Sprintf("ServeHTTP %v %v %v", m, record, clientIP))
		}

	}
	return next.ServeHTTP(w, r)
}

func getClientIP(r *http.Request, strict bool) (net.IP, error) {
	var ip string

	// Use the client ip from the 'X-Forwarded-For' header, if available.
	if fwdFor := r.Header.Get("X-Forwarded-For"); fwdFor != "" && !strict {
		ips := strings.Split(fwdFor, ", ")
		ip = ips[0]
	} else {
		// Otherwise, get the client ip from the request remote address.
		var err error
		ip, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			if serr, ok := err.(*net.AddrError); ok && serr.Err == "missing port in address" { // It's not critical try parse
				ip = r.RemoteAddr
			} else {
				log.Printf("Error when SplitHostPort: %v", serr.Err)
				return nil, err
			}
		}
	}

	// Parse the ip address string into a net.IP.
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, errors.New("unable to parse address")
	}

	return parsedIP, nil
}

// for http handler
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m GeoIP2
	// err := m.UnmarshalCaddyfile(h.Dispenser)
	d := h.Dispenser

	for d.Next() {
		if !d.Args(&m.Enable) {
			return nil, d.ArgErr()
		}
	}
	return m, nil
}

func (g *GeoIP2) Provision(ctx caddy.Context) error {
	// TODO: set up the module
	caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("Provision"))
	return nil
}
func (g GeoIP2) Validate() error {
	// TODO: validate the module's setup
	caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("Validate"))
	return nil
}

// Interface guards
var (
	_ caddy.Module                = (*GeoIP2)(nil)
	_ caddy.Provisioner           = (*GeoIP2)(nil)
	_ caddy.Validator             = (*GeoIP2)(nil)
	_ caddyhttp.MiddlewareHandler = (*GeoIP2)(nil)
)
