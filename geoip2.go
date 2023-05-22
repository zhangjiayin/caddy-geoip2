package geoip2

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
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

// http.handlers.geoip2 is an GeoIP2 server handler.
// it uses GeoIP2 Data to identify the location of the IP
type GeoIP2 struct {
	// strict: only use remote IP address
	// wild: use X-Forwarded-For if it exists
	// trusted_proxies: use X-Forwarded-For if exists when trusted_proxies if valid
	// default:trusted_proxies
	Enable string        `json:"enable,omitempty"`
	state  *GeoIP2State  `json:"-"`
	ctx    caddy.Context `json:"-"`
}

type IpSafeLevel int

const (
	Wild           IpSafeLevel = 0
	TrustedProxies IpSafeLevel = 1
	Strict         IpSafeLevel = 100
)

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
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	//init some variables with default value ""
	repl.Set("geoip2.ip_address", "")
	repl.Set("geoip2.country_code", "")
	repl.Set("geoip2.country_name", "")
	repl.Set("geoip2.country_eu", "")
	repl.Set("geoip2.country_locales", "")
	repl.Set("geoip2.country_confidence", "")
	repl.Set("geoip2.country_names", "")
	repl.Set("geoip2.country_geoname_id", "")
	repl.Set("geoip2.continent_code", "")
	repl.Set("geoip2.continent_locales", "")
	repl.Set("geoip2.continent_names", "")
	repl.Set("geoip2.continent_geoname_id", "")
	repl.Set("geoip2.continent_name", "")
	repl.Set("geoip2.city_confidence", "")
	repl.Set("geoip2.city_locales", "")
	repl.Set("geoip2.city_names", "")
	repl.Set("geoip2.city_geoname_id", "")
	// repl.Set("geoip2.city_name", val)
	repl.Set("geoip2.city_name", "")
	repl.Set("geoip2.location_latitude", "")
	repl.Set("geoip2.location_longitude", "")
	repl.Set("geoip2.location_time_zone", "")
	repl.Set("geoip2.location_accuracy_radius", "")
	repl.Set("geoip2.location_average_income", "")
	repl.Set("geoip2.location_metro_code", "")
	repl.Set("geoip2.location_population_density", "")
	repl.Set("geoip2.postal_code", "")
	repl.Set("geoip2.postal_confidence", "")
	repl.Set("geoip2.registeredcountry_geoname_id", "")
	repl.Set("geoip2.registeredcountry_is_in_european_union", "")
	repl.Set("geoip2.registeredcountry_iso_code", "")
	repl.Set("geoip2.registeredcountry_names", "")

	repl.Set("geoip2.registeredcountry_name", "")
	repl.Set("geoip2.representedcountry_geoname_id", "")
	repl.Set("geoip2.representedcountry_is_in_european_union", "")
	repl.Set("geoip2.representedcountry_iso_code", "")
	repl.Set("geoip2.representedcountry_names", "")
	repl.Set("geoip2.representedcountry_locales", "")
	repl.Set("geoip2.representedcountry_confidence", "")
	repl.Set("geoip2.representedcountry_type", "")
	repl.Set("geoip2.representedcountry_name", "")
	repl.Set("geoip2.subdivisions", "")
	repl.Set("geoip2.traits_is_anonymous_proxy", "")
	repl.Set("geoip2.traits_is_anonymous_vpn", "")
	repl.Set("geoip2.traits_is_satellite_provider", "")
	repl.Set("geoip2.traits_autonomous_system_number", "")
	repl.Set("geoip2.traits_autonomous_system_organization", "")
	repl.Set("geoip2.traits_connection_type", "")
	repl.Set("geoip2.traits_domain", "")
	repl.Set("geoip2.traits_is_hosting_provider", "")
	repl.Set("geoip2.traits_is_legitimate_proxy", "")
	repl.Set("geoip2.traits_is_public_proxy", "")
	repl.Set("geoip2.traits_is_residential_proxy", "")
	repl.Set("geoip2.traits_is_tor_exit_node", "")
	repl.Set("geoip2.traits_isp", "")
	repl.Set("geoip2.traits_mobile_country_code", "")
	repl.Set("geoip2.traits_mobile_network_code", "")
	repl.Set("geoip2.traits_network", "")
	repl.Set("geoip2.traits_organization", "")
	repl.Set("geoip2.traits_user_type", "")
	repl.Set("geoip2.traits_userCount", "")
	repl.Set("geoip2.traits_static_ip_score", "")

	if m.Enable != "off" && m.Enable != "false" && m.Enable != "0" {
		var record = GeoIP2Record{}
		if m.state != nil && m.state.DBHandler != nil {

			clientIP, _ := m.getClientIP(r)
			m.state.DBHandler.Lookup(clientIP, &record)

			if clientIP == nil {
				repl.Set("geoip2.ip_address", "")
			} else {
				repl.Set("geoip2.ip_address", clientIP.String())
			}

			//country
			repl.Set("geoip2.country_code", record.Country.ISOCode)

			for key, element := range record.Country.Names {
				repl.Set("geoip2.country_names_"+key, element)
				if key == "en" {
					repl.Set("geoip2.country_name", element)
				}
			}

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

			for key, element := range record.Continent.Names {
				repl.Set("geoip2.continent_names_"+key, element)
				if key == "en" {
					repl.Set("geoip2.continent_name", element)
				}
			}

			//City
			repl.Set("geoip2.city_confidence", record.City.Confidence)
			repl.Set("geoip2.city_locales", record.City.Locales)
			repl.Set("geoip2.city_names", record.City.Names)
			repl.Set("geoip2.city_geoname_id", record.City.GeoNameID)
			// val, _ = record.City.Names["en"]
			// repl.Set("geoip2.city_name", val)

			for key, element := range record.City.Names {
				repl.Set("geoip2.city_names_"+key, element)
				if key == "en" {
					repl.Set("geoip2.city_name", element)
				}
			}

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
			// val, _ = record.RegisteredCountry.Names["en"]
			// repl.Set("geoip2.registeredcountry_name", val)

			for key, element := range record.RegisteredCountry.Names {
				repl.Set("geoip2.registeredcountry_names_"+key, element)
				if key == "en" {
					repl.Set("geoip2.registeredcountry_name", element)
				}
			}

			//RepresentedCountry
			repl.Set("geoip2.representedcountry_geoname_id", record.RepresentedCountry.GeoNameID)
			repl.Set("geoip2.representedcountry_is_in_european_union", record.RepresentedCountry.IsInEuropeanUnion)
			repl.Set("geoip2.representedcountry_iso_code", record.RepresentedCountry.IsoCode)
			repl.Set("geoip2.representedcountry_names", record.RepresentedCountry.Names)
			repl.Set("geoip2.representedcountry_locales", record.RepresentedCountry.Locales)
			repl.Set("geoip2.representedcountry_confidence", record.RepresentedCountry.Confidence)
			repl.Set("geoip2.representedcountry_type", record.RepresentedCountry.Type)
			// val, _ = record.RepresentedCountry.Names["en"]
			// repl.Set("geoip2.representedcountry_name", val)

			for key, element := range record.RepresentedCountry.Names {
				repl.Set("geoip2.representedcountry_names_"+key, element)
				if key == "en" {
					repl.Set("geoip2.representedcountry_name", element)
				}
			}

			repl.Set("geoip2.subdivisions", record.Subdivisions)

			for index, subdivision := range record.Subdivisions {
				indexStr := strconv.Itoa(index + 1)
				repl.Set("geoip2.subdivisions_"+indexStr+"_confidence", subdivision.Confidence)
				repl.Set("geoip2.subdivisions_"+indexStr+"_geoname_id", subdivision.GeoNameID)
				repl.Set("geoip2.subdivisions_"+indexStr+"_iso_code", subdivision.IsoCode)
				repl.Set("geoip2.subdivisions_"+indexStr+"_locales", subdivision.Locales)
				repl.Set("geoip2.subdivisions_"+indexStr+"_names", subdivision.Names)
				for key, element := range subdivision.Locales {
					keyStr := strconv.Itoa(key)
					repl.Set("geoip2.subdivisions_"+indexStr+"_locales_"+keyStr, element)
				}
				for key, element := range subdivision.Names {
					repl.Set("geoip2.subdivisions_"+indexStr+"_names_"+key, element)
					if key == "en" {
						repl.Set("geoip2.subdivisions_"+indexStr+"_name", element)
					}
				}
			}

			//Traits
			repl.Set("geoip2.traits_is_anonymous_proxy", record.Traits.IsAnonymousProxy)
			repl.Set("geoip2.traits_is_anonymous_vpn", record.Traits.IsAnonymousVpn)
			repl.Set("geoip2.traits_is_satellite_provider", record.Traits.IsSatelliteProvider)
			repl.Set("geoip2.traits_autonomous_system_number", record.Traits.AutonomousSystemNumber)
			repl.Set("geoip2.traits_autonomous_system_organization", record.Traits.AutonomousSystemOrganization)

			//Traits
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

func (m GeoIP2) getClientIP(r *http.Request) (net.IP, error) {
	var ip string

	trustedProxy := caddyhttp.GetVar(r.Context(), caddyhttp.TrustedProxyVarKey).(bool)

	safeLevel := TrustedProxies

	if strings.ToLower(m.Enable) == "strict" {
		safeLevel = Strict
	} else if strings.ToLower(m.Enable) == "wild" {
		safeLevel = Wild
	}

	fwdFor := r.Header.Get("X-Forwarded-For")

	if ((safeLevel == TrustedProxies && trustedProxy) || safeLevel == Wild) && fwdFor != "" {
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
	app, err := ctx.App(moduleName)
	if err != nil {
		return fmt.Errorf("getting geoip2 app: %v", err)
	}
	g.state = app.(*GeoIP2State)
	g.ctx = ctx
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
