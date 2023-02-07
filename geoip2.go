package geoip2

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type GeoIP2Record struct {
	Country struct {
		Locales           []string          `json:"locales"`
		Confidence        uint16            `maxminddb:"confidence"`
		ISOCode           string            `maxminddb:"iso_code"`
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
		Names             map[string]string `maxminddb:"names"`
		GeoNameID         uint64            `maxminddb:"geoname_id"`
	} `maxminddb:"country"`

	Continent struct {
		Locales   []string          `json:"locales"`
		Code      string            `maxminddb:"code"`
		GeoNameID uint              `maxminddb:"geoname_id"`
		Names     map[string]string `maxminddb:"names"`
	} `maxminddb:"continent"`

	City struct {
		Names      map[string]string `maxminddb:"names"`
		Confidence uint16            `maxminddb:"confidence"`
		GeoNameID  uint64            `maxminddb:"geoname_id"`
		Locales    []string          `json:"locales"`
	} `maxminddb:"city"`

	Location struct {
		AccuracyRadius    uint16  `maxminddb:"accuracy_radius"`
		AverageIncome     uint16  `maxminddb:"average_income"`
		Latitude          float64 `maxminddb:"latitude"`
		Longitude         float64 `maxminddb:"longitude"`
		MetroCode         uint    `maxminddb:"metro_code"`
		PopulationDensity uint    `maxminddb:"population_density"`
		TimeZone          string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`

	Postal struct {
		Code       string `maxminddb:"code"`
		Confidence uint16 `maxminddb:"confidence"`
	} `maxminddb:"postal"`

	RegisteredCountry struct {
		GeoNameID         uint              `maxminddb:"geoname_id"`
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
		IsoCode           string            `maxminddb:"iso_code"`
		Names             map[string]string `maxminddb:"names"`
	} `maxminddb:"registered_country"`

	RepresentedCountry struct {
		Locales           []string          `json:"locales"`
		Confidence        uint16            `maxminddb:"confidence"`
		GeoNameID         uint              `maxminddb:"geoname_id"`
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
		IsoCode           string            `maxminddb:"iso_code"`
		Names             map[string]string `maxminddb:"names"`
		Type              string            `maxminddb:"type"`
	} `maxminddb:"represented_country"`

	Subdivisions []struct {
		Locales    []string          `json:"locales"`
		Confidence uint16            `maxminddb:"confidence"`
		GeoNameID  uint              `maxminddb:"geoname_id"`
		IsoCode    string            `maxminddb:"iso_code"`
		Names      map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`

	Traits struct {
		IsAnonymousProxy    bool `maxminddb:"is_anonymous_proxy"`
		IsAnonymousVpn      bool `maxminddb:"is_anonymous_vpn"`
		IsSatelliteProvider bool `maxminddb:"is_satellite_provider"`

		AutonomousSystemNumber       uint64 `maxminddb:"autonomous_system_number"`
		AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
		ConnectionType               string `maxminddb:"connection_type"`
		Domain                       string `maxminddb:"domain"`

		IsHostingProvider  bool    `maxminddb:"is_hosting_provider"`
		IsLegitimateProxy  bool    `maxminddb:"is_legitimate_proxy"`
		IsPublicProxy      bool    `maxminddb:"is_public_proxy"`
		IsResidentialProxy bool    `maxminddb:"is_residential_proxy"`
		IsTorExitNode      bool    `maxminddb:"is_tor_exit_node"`
		Isp                string  `maxminddb:"isp"`
		MobileCountryCode  string  `maxminddb:"mobile_country_code"`
		MobileNetworkCode  string  `maxminddb:"mobile_network_code"`
		Network            string  `maxminddb:"network"`
		Organization       string  `maxminddb:"organization"`
		UserType           string  `maxminddb:"user_type"`
		UserCount          int32   `maxminddb:"userCount"`
		StaticIpScore      float64 `maxminddb:"static_ip_score"`
	} `maxminddb:"traits"`
}

type GeoIP2 struct {
	Enable string `json:"enable,omitempty"`
}

func init() {
	caddy.RegisterModule(GeoIP2{})
	httpcaddyfile.RegisterHandlerDirective("geoip2_vars", parseCaddyfile)
}

func (GeoIP2) CaddyModule() caddy.ModuleInfo {
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

			repl.Set("geoip2.ip_address", clientIP.String())
			//country
			repl.Set("geoip2.country_code", record.Country.ISOCode)
			val, _ := record.Country.Names["en"]
			repl.Set("geoip2.country_name", val)
			repl.Set("geoip2.country_eu", record.Country.IsInEuropeanUnion)
			repl.Set("geoip2.country_locales", record.Country.Locales)
			repl.Set("geoip2.country_confidence", record.Country.Confidence)
			repl.Set("geoip2.country_names", record.Country.Names)
			repl.Set("geoip2.country_geoname_id", record.Country.GeoNameID)

			//Continent
			repl.Set("geoip2.continent_code", record.Continent.Code)
			repl.Set("geoip2.continent_locales", record.Continent.Locales)
			repl.Set("geoip2.continent_names", record.Continent.Names)
			repl.Set("geoip2.continent_geoname_id", record.Continent.GeoNameID)
			val, _ = record.Continent.Names["en"]
			repl.Set("geoip2.continent_name", val)

			//City
			repl.Set("geoip2.city_confidence", record.City.Confidence)
			repl.Set("geoip2.city_locales", record.City.Locales)
			repl.Set("geoip2.city_names", record.City.Names)
			repl.Set("geoip2.city_geoname_id", record.City.GeoNameID)
			val, _ = record.City.Names["en"]
			repl.Set("geoip2.city_name", val)

			//Location
			repl.Set("geoip2.location_latitude", record.Location.Latitude)
			repl.Set("geoip2.location_longitude", record.Location.Longitude)
			repl.Set("geoip2.location_time_zone", record.Location.TimeZone)
			repl.Set("geoip2.location_accuracy_radius", record.Location.AccuracyRadius)
			repl.Set("geoip2.location_average_income", record.Location.AverageIncome)
			repl.Set("geoip2.location_metro_code", record.Location.MetroCode)
			repl.Set("geoip2.location_population_density", record.Location.PopulationDensity)

			//Postal
			repl.Set("geoip2.postal_code", record.Postal.Code)
			repl.Set("geoip2.postal_confidence", record.Postal.Confidence)

			//RegisteredCountry
			repl.Set("geoip2.registeredcountry_geoname_id", record.RegisteredCountry.GeoNameID)
			repl.Set("geoip2.registeredcountry_is_in_european_union", record.RegisteredCountry.IsInEuropeanUnion)
			repl.Set("geoip2.registeredcountry_iso_code", record.RegisteredCountry.IsoCode)
			repl.Set("geoip2.registeredcountry_names", record.RegisteredCountry.Names)
			val, _ = record.RegisteredCountry.Names["en"]
			repl.Set("geoip2.registeredcountry_name", val)

			//RepresentedCountry
			repl.Set("geoip2.representedcountry_geoname_id", record.RepresentedCountry.GeoNameID)
			repl.Set("geoip2.representedcountry_is_in_european_union", record.RepresentedCountry.IsInEuropeanUnion)
			repl.Set("geoip2.representedcountry_iso_code", record.RepresentedCountry.IsoCode)
			repl.Set("geoip2.representedcountry_names", record.RepresentedCountry.Names)
			repl.Set("geoip2.representedcountry_locales", record.RepresentedCountry.Locales)
			repl.Set("geoip2.representedcountry_confidence", record.RepresentedCountry.Confidence)
			repl.Set("geoip2.representedcountry_type", record.RepresentedCountry.Type)
			val, _ = record.RepresentedCountry.Names["en"]
			repl.Set("geoip2.representedcountry_name", val)

			//Traits
			repl.Set("geoip2.traits_is_anonymous_proxy", record.Traits.IsAnonymousProxy)
			repl.Set("geoip2.traits_is_anonymous_vpn", record.Traits.IsAnonymousVpn)
			repl.Set("geoip2.traits_is_satellite_provider", record.Traits.IsSatelliteProvider)
			repl.Set("geoip2.traits_autonomous_system_number", record.Traits.AutonomousSystemNumber)
			repl.Set("geoip2.traits_autonomous_system_organization", record.Traits.AutonomousSystemOrganization)

			//Traits
			repl.Set("geoip2.traits_autonomous_system_organization", record.Traits.AutonomousSystemOrganization)
			repl.Set("geoip2.traits_autonomous_system_organization", record.Traits.AutonomousSystemOrganization)
			repl.Set("geoip2.traits_connection_type", record.Traits.ConnectionType)
			repl.Set("geoip2.traits_domain", record.Traits.Domain)
			repl.Set("geoip2.traits_is_hosting_provider", record.Traits.IsHostingProvider)
			repl.Set("geoip2.traits_is_legitimate_proxy", record.Traits.IsLegitimateProxy)
			repl.Set("geoip2.traits_is_public_proxy", record.Traits.IsPublicProxy)
			repl.Set("geoip2.traits_is_residential_proxy", record.Traits.IsResidentialProxy)
			repl.Set("geoip2.traits_is_tor_exit_node", record.Traits.IsTorExitNode)
			repl.Set("geoip2.traits_isp", record.Traits.Isp)
			repl.Set("geoip2.traits_mobile_country_code", record.Traits.MobileCountryCode)
			repl.Set("geoip2.traits_mobile_network_code", record.Traits.MobileNetworkCode)
			repl.Set("geoip2.traits_network", record.Traits.Network)
			repl.Set("geoip2.traits_organization", record.Traits.Organization)
			repl.Set("geoip2.traits_user_type", record.Traits.UserType)
			repl.Set("geoip2.traits_userCount", record.Traits.UserCount)
			repl.Set("geoip2.traits_static_ip_score", record.Traits.StaticIpScore)

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

func (g *GeoIP2) Provision(ctx caddy.Context) error {
	caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("Provision"))
	return nil
}
func (g GeoIP2) Validate() error {
	caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("Validate"))
	return nil
}

// Interface guards
var (
	_ caddy.Module                = (*GeoIP2)(nil)
	_ caddy.Provisioner           = (*GeoIP2)(nil)
	_ caddy.Validator             = (*GeoIP2)(nil)
	_ caddyhttp.MiddlewareHandler = (*GeoIP2)(nil)
	_ caddyfile.Unmarshaler       = (*GeoIP2)(nil)
)
