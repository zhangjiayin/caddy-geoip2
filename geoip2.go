package geoip2

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

// CountryRecord defines the structure for Country database lookups
// Contains country-specific information including EU membership status
type CountryRecord struct {
	Country struct {
		ISOCode           string `maxminddb:"iso_code"`             // Two-letter country code (e.g., "DE", "US")
		IsInEuropeanUnion bool   `maxminddb:"is_in_european_union"` // Whether country is in EU
	} `maxminddb:"country"`

	RegisteredCountry struct {
		IsInEuropeanUnion bool `maxminddb:"is_in_european_union"` // Whether registered country is in EU
	} `maxminddb:"registered_country"`
}

// CityRecord defines the structure for City database lookups
// Contains city, subdivision, and location information
type CityRecord struct {
	City struct {
		Names map[string]string `maxminddb:"names"` // City names in different languages
	} `maxminddb:"city"`

	Location struct {
		Latitude  float64 `maxminddb:"latitude"`  // Geographic latitude
		Longitude float64 `maxminddb:"longitude"` // Geographic longitude
	} `maxminddb:"location"`

	Subdivisions []struct {
		IsoCode string `maxminddb:"iso_code"` // State/Province code (e.g., "CA", "BY")
	} `maxminddb:"subdivisions"`
}

// ASNRecord defines the structure for ASN database lookups
// Used for autonomous system information
type ASNRecord struct {
	AutonomousSystemNumber       uint64 `maxminddb:"autonomous_system_number"`       // ASN number
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"` // ASN organization name
}

// GeoIP2 is the HTTP middleware handler that provides GeoIP2 functionality
// It enriches requests with geographic information based on client IP
type GeoIP2 struct {
	// Enable controls the IP detection mode:
	// - "strict": only use remote IP address (ignore X-Forwarded-For)
	// - "wild": trust X-Forwarded-For header unconditionally
	// - "trusted_proxies": trust X-Forwarded-For only from trusted proxies (default)
	// - "client_ip": use the client IP Caddy determined itself (trusted_proxies,
	//   trusted_proxies_strict and client_ip_headers of the server)
	// - "off"/"false"/"0": disable GeoIP2 lookups
	Enable string `json:"enable,omitempty"`

	// state holds reference to the shared GeoIP2 database state
	state *GeoIP2State `json:"-"`

	// ctx is the Caddy context for this module instance
	ctx caddy.Context `json:"-"`
}

// IpSafeLevel defines the security level for IP address detection
type IpSafeLevel int

const (
	Wild           IpSafeLevel = 0   // Trust any X-Forwarded-For header
	TrustedProxies IpSafeLevel = 1   // Only trust X-Forwarded-For from trusted proxies
	ClientIP       IpSafeLevel = 2   // Use the client IP Caddy determined for the request
	Strict         IpSafeLevel = 100 // Never trust X-Forwarded-For, use RemoteAddr only
)

// Variable names that will be set in Caddy's replacer
// Using underscore notation instead of dots for better compatibility
const (
	VarCity         = "geoip2_city"
	VarCountryCode  = "geoip2_country_code"
	VarLatitude     = "geoip2_latitude"
	VarLongitude    = "geoip2_longitude"
	VarSubdivisions = "geoip2_subdivisions"
	VarIsInEU       = "geoip2_is_in_eu"
	VarASN          = "geoip2_asn"
	VarASOrg        = "geoip2_asorg"
)

// Module registration - called when Caddy starts
func init() {
	caddy.RegisterModule(GeoIP2{})
	httpcaddyfile.RegisterHandlerDirective("geoip2_vars", parseCaddyfile)
}

// CaddyModule returns module information for Caddy's module system
func (GeoIP2) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.geoip2",
		New: func() caddy.Module { return new(GeoIP2) },
	}
}

// ServeHTTP implements the HTTP middleware interface
// This is called for every HTTP request that passes through this middleware
func (m GeoIP2) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// Get Caddy's replacer to set variables that can be used in config
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	// Initialize all GeoIP2 variables with empty defaults
	// This ensures they're always available even if lookup fails
	m.initializeVariables(repl)

	// Only perform lookup if GeoIP2 is enabled
	if m.isEnabled() {
		// Try to perform GeoIP2 lookup and populate variables
		m.performLookup(r, repl)
	}

	// Continue to next handler in chain
	return next.ServeHTTP(w, r)
}

// initializeVariables sets all GeoIP2 variables to empty defaults
// This prevents undefined variable errors in Caddy config
func (m *GeoIP2) initializeVariables(repl *caddy.Replacer) {
	repl.Set(VarCity, "")
	repl.Set(VarCountryCode, "")
	repl.Set(VarLatitude, "")
	repl.Set(VarLongitude, "")
	repl.Set(VarSubdivisions, "")
	repl.Set(VarIsInEU, "")
	repl.Set(VarASN, "")
	repl.Set(VarASOrg, "")
}

// isEnabled checks if GeoIP2 lookups should be performed
func (m *GeoIP2) isEnabled() bool {
	return m.Enable != "off" && m.Enable != "false" && m.Enable != "0"
}

// performLookup does the actual GeoIP2 database lookups and sets variables
// Implements intelligent routing: EU IPs use Europe-specific city database,
// non-EU IPs use global city database for optimal performance
func (m *GeoIP2) performLookup(r *http.Request, repl *caddy.Replacer) {
	// Check if databases are available
	if m.state == nil {
		caddy.Log().Named("http.handlers.geoip2").Warn("GeoIP2 state not available")
		return
	}

	// Get client IP address based on configured safety level
	clientIP, err := m.getClientIP(r)
	if err != nil {
		caddy.Log().Named("http.handlers.geoip2").Debug("failed to get client IP",
			zap.Error(err))
		return
	}

	// Perform Country database lookup first (needed for EU routing decision)
	var countryRecord CountryRecord
	var countryCode string
	var isInEU bool
	if m.state.CountryDBHandler != nil {
		if err := m.state.Lookup(clientIP, &countryRecord); err != nil {
			caddy.Log().Named("http.handlers.geoip2").Debug("Country lookup failed",
				zap.String("ip", clientIP.String()),
				zap.Error(err))
		} else {
			countryCode = countryRecord.Country.ISOCode
			// Check both country and registered_country for EU status
			isInEU = countryRecord.Country.IsInEuropeanUnion || countryRecord.RegisteredCountry.IsInEuropeanUnion
		}
	}

	// Perform intelligent City database lookup based on EU status
	var cityRecord CityRecord
	var cityName string
	var latitude, longitude float64
	var subdivision string

	// Decide which city database to use based on EU status
	var cityLookupFunc func(interface{}, interface{}) error
	var dbName string

	if isInEU && m.state.CityDBHandler != nil {
		// EU IP: Use Europe-specific database
		cityLookupFunc = m.state.LookupCity
		dbName = "Europe city database"
	} else if m.state.GlobalCityDBHandler != nil {
		// Non-EU IP: Use global database as fallback
		cityLookupFunc = m.state.LookupGlobalCity
		dbName = "Global city database"
	}

	if cityLookupFunc != nil {
		if err := cityLookupFunc(clientIP, &cityRecord); err != nil {
			caddy.Log().Named("http.handlers.geoip2").Debug("City lookup failed",
				zap.String("ip", clientIP.String()),
				zap.String("database", dbName),
				zap.Bool("is_eu", isInEU),
				zap.Error(err))
		} else {
			// Extract city name (prefer German as specified in nginx config, fallback to English, then any)
			if name, exists := cityRecord.City.Names["de"]; exists && name != "" {
				cityName = name
			} else if name, exists := cityRecord.City.Names["en"]; exists && name != "" {
				cityName = name
			} else {
				// If no German or English name, try to get any available city name
				for _, name := range cityRecord.City.Names {
					if name != "" {
						cityName = name
						break
					}
				}
			}

			// Extract location data
			latitude = cityRecord.Location.Latitude
			longitude = cityRecord.Location.Longitude

			// Extract subdivision (state/province) - use first available
			if len(cityRecord.Subdivisions) > 0 && cityRecord.Subdivisions[0].IsoCode != "" {
				subdivision = cityRecord.Subdivisions[0].IsoCode
			}

			caddy.Log().Named("http.handlers.geoip2").Debug("City lookup successful",
				zap.String("ip", clientIP.String()),
				zap.String("database", dbName),
				zap.Bool("is_eu", isInEU),
				zap.String("city", cityName))
		}
	}

	// Perform ASN database lookup
	var asnRecord ASNRecord
	var asnNumber uint64
	var asnOrg string
	if m.state.ASNDBHandler != nil {
		if err := m.state.LookupASN(clientIP, &asnRecord); err != nil {
			caddy.Log().Named("http.handlers.geoip2").Debug("ASN lookup failed",
				zap.String("ip", clientIP.String()),
				zap.Error(err))
		} else {
			asnNumber = asnRecord.AutonomousSystemNumber
			asnOrg = asnRecord.AutonomousSystemOrganization
		}
	}

	// Set all GeoIP2 variables with the combined results
	repl.Set(VarCountryCode, countryCode)
	repl.Set(VarIsInEU, isInEU)
	repl.Set(VarCity, cityName)
	repl.Set(VarLatitude, latitude)
	repl.Set(VarLongitude, longitude)
	repl.Set(VarSubdivisions, subdivision)
	repl.Set(VarASN, asnNumber)
	repl.Set(VarASOrg, asnOrg)

	// Debug logging with performance information
	caddy.Log().Named("http.handlers.geoip2").Debug("GeoIP2 lookups completed",
		zap.String("ip", clientIP.String()),
		zap.String("country", countryCode),
		zap.String("city", cityName),
		zap.Bool("is_in_eu", isInEU),
		zap.String("city_database_used", dbName),
		zap.Uint64("asn", asnNumber))
}

// getClientIP determines the real client IP address based on configuration
// Handles X-Forwarded-For header according to security settings
func (m GeoIP2) getClientIP(r *http.Request) (net.IP, error) {
	var ipStr string

	// Determine if we're behind a trusted proxy
	trustedProxy, _ := caddyhttp.GetVar(r.Context(), caddyhttp.TrustedProxyVarKey).(bool)

	// Convert string setting to safety level
	safeLevel := m.getSafetyLevel()

	// Get X-Forwarded-For header if present
	forwardedFor := r.Header.Get("X-Forwarded-For")

	// The client IP Caddy determined before any handler ran. For a trusted
	// proxy it comes from the client IP headers, read the way the server is
	// configured (with trusted_proxies_strict from the right, skipping
	// trusted proxies); for anyone else it is the connection's address.
	// Empty when Caddy could not parse the connection's address.
	caddyClientIP, _ := caddyhttp.GetVar(r.Context(), caddyhttp.ClientIPVarKey).(string)

	// Decide which IP to use based on safety level and proxy trust
	if safeLevel == ClientIP && caddyClientIP != "" {
		ipStr = caddyClientIP
	} else if ((safeLevel == TrustedProxies && trustedProxy) || safeLevel == Wild) && forwardedFor != "" {
		// Use X-Forwarded-For header (take first IP in chain).
		// The first entry is the one the client itself can write: a trusted
		// proxy appends to the header, it does not replace it. Use
		// "client_ip" when the result must not be forgeable.
		ips := strings.Split(forwardedFor, ", ")
		ipStr = strings.TrimSpace(ips[0])
	} else {
		// Use direct connection IP from RemoteAddr
		var err error
		ipStr, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			// Handle case where RemoteAddr doesn't have port
			if serr, ok := err.(*net.AddrError); ok && serr.Err == "missing port in address" {
				ipStr = r.RemoteAddr
			} else {
				log.Printf("Error parsing RemoteAddr: %v", err)
				return nil, err
			}
		}
	}

	// Parse and validate IP address
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	return parsedIP, nil
}

// getSafetyLevel converts string configuration to IpSafeLevel enum
func (m *GeoIP2) getSafetyLevel() IpSafeLevel {
	switch strings.ToLower(m.Enable) {
	case "strict":
		return Strict
	case "wild":
		return Wild
	case "client_ip":
		return ClientIP
	default:
		return TrustedProxies
	}
}

// parseCaddyfile parses the Caddyfile directive for this handler
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m GeoIP2
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler
// Parses: geoip2_vars <mode>
func (m *GeoIP2) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		// Parse the mode argument (strict/wild/trusted_proxies/client_ip)
		if !d.Args(&m.Enable) {
			return d.ArgErr()
		}
	}
	return nil
}

// Provision sets up the module with Caddy context
// Links this handler to the shared GeoIP2 database state
func (g *GeoIP2) Provision(ctx caddy.Context) error {
	caddy.Log().Named("http.handlers.geoip2").Debug("provisioning GeoIP2 handler")

	// Get reference to the shared GeoIP2 app/state
	app, err := ctx.App(moduleName)
	if err != nil {
		return fmt.Errorf("getting geoip2 app: %v", err)
	}

	// Store reference to shared state
	g.state = app.(*GeoIP2State)
	g.ctx = ctx

	return nil
}

// Validate checks if the configuration is valid
func (g GeoIP2) Validate() error {
	caddy.Log().Named("http.handlers.geoip2").Debug("validating GeoIP2 handler")

	// Validate Enable setting
	validModes := []string{"strict", "wild", "trusted_proxies", "client_ip", "off", "false", "0", ""}
	mode := strings.ToLower(g.Enable)
	for _, valid := range validModes {
		if mode == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid enable mode '%s', must be one of: %v", g.Enable, validModes)
}

// Interface guards - compile-time checks that we implement required interfaces
var (
	_ caddy.Module                = (*GeoIP2)(nil)
	_ caddy.Provisioner           = (*GeoIP2)(nil)
	_ caddy.Validator             = (*GeoIP2)(nil)
	_ caddyhttp.MiddlewareHandler = (*GeoIP2)(nil)
	_ caddyfile.Unmarshaler       = (*GeoIP2)(nil)
)
