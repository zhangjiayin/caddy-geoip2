package replacer

import (
	"fmt"
	"net"
	"strconv"

	"github.com/caddyserver/caddy/v2"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
	"go.uber.org/zap"
)

// Enterprise represents a geoip reader for Enterprise,
// City and Country type databases.
type Enterprise struct {
	reader *maxminddb.Reader
}

// Lookup performs a database lookup on the provided clientIP and sets
// the related replacer variables.
func (r *Enterprise) Lookup(repl *caddy.Replacer, clientIP net.IP) {
	var record geoip2.Enterprise
	err := r.reader.Lookup(clientIP, &record)
	if err != nil {
		caddy.Log().Named("geoip2").
			Error(fmt.Sprintf(
				"looking up Enterprise record for IP %q: %+v",
				clientIP.String(),
				err,
			))
	}

	SetEnterprise(repl, record)

	caddy.Log().Named("http.handlers.geoip2").
		Debug("Lookup Enterprise/City/Country", zap.String("clientIP", clientIP.String()), zap.Any("record", record))
}

// Close closes the database reader.
func (r *Enterprise) Close() error {
	return r.reader.Close()
}

// languageCodes is the list of languages used for names.
var languageCodes = []string{"de", "en", "es", "fr", "ja", "pt-BR", "ru", "zh-CN"}

// subdivisionSize is the minimum size of a subdivision slice.
const subdivisionSize = 2

// SetEnterprise sets values for possible replacer variables.
func SetEnterprise(repl *caddy.Replacer, record geoip2.Enterprise) {
	// country
	repl.Set("geoip2.country_code", record.Country.IsoCode)
	repl.Set("geoip2.country_confidence", record.Country.Confidence)
	repl.Set("geoip2.country_eu", record.Country.IsInEuropeanUnion)
	repl.Set("geoip2.country_geoname_id", record.Country.GeoNameID)
	repl.Set("geoip2.country_names", record.Country.Names)
	for _, lc := range languageCodes {
		value := record.Country.Names[lc]
		repl.Set("geoip2.country_names_"+lc, value)
		if lc == "en" {
			repl.Set("geoip2.country_name", value)
		}
	}

	// Continent
	repl.Set("geoip2.continent_code", record.Continent.Code)
	repl.Set("geoip2.continent_names", record.Continent.Names)
	repl.Set("geoip2.continent_geoname_id", record.Continent.GeoNameID)
	for _, lc := range languageCodes {
		value := record.Continent.Names[lc]
		repl.Set("geoip2.continent_names_"+lc, value)
		if lc == "en" {
			repl.Set("geoip2.continent_name", value)
		}
	}

	// City
	repl.Set("geoip2.city_confidence", record.City.Confidence)
	repl.Set("geoip2.city_geoname_id", record.City.GeoNameID)
	repl.Set("geoip2.city_names", record.City.Names)
	for _, lc := range languageCodes {
		value := record.City.Names[lc]
		repl.Set("geoip2.city_names_"+lc, value)
		if lc == "en" {
			repl.Set("geoip2.city_name", value)
		}
	}

	// Location
	repl.Set("geoip2.location_latitude", record.Location.Latitude)
	repl.Set("geoip2.location_longitude", record.Location.Longitude)
	repl.Set("geoip2.location_time_zone", record.Location.TimeZone)
	repl.Set("geoip2.location_accuracy_radius", record.Location.AccuracyRadius)
	repl.Set("geoip2.location_metro_code", record.Location.MetroCode)

	// Postal
	repl.Set("geoip2.postal_code", record.Postal.Code)
	repl.Set("geoip2.postal_confidence", record.Postal.Confidence)

	// RegisteredCountry
	repl.Set("geoip2.registeredcountry_geoname_id", record.RegisteredCountry.GeoNameID)
	repl.Set(
		"geoip2.registeredcountry_is_in_european_union",
		record.RegisteredCountry.IsInEuropeanUnion,
	)
	repl.Set("geoip2.registeredcountry_iso_code", record.RegisteredCountry.IsoCode)
	repl.Set("geoip2.registeredcountry_names", record.RegisteredCountry.Names)
	for _, lc := range languageCodes {
		value := record.RegisteredCountry.Names[lc]
		repl.Set("geoip2.registeredcountry_names_"+lc, value)
		if lc == "en" {
			repl.Set("geoip2.registeredcountry_name", value)
		}
	}

	// RepresentedCountry
	repl.Set("geoip2.representedcountry_geoname_id", record.RepresentedCountry.GeoNameID)
	repl.Set(
		"geoip2.representedcountry_is_in_european_union",
		record.RepresentedCountry.IsInEuropeanUnion,
	)
	repl.Set("geoip2.representedcountry_iso_code", record.RepresentedCountry.IsoCode)
	repl.Set("geoip2.representedcountry_names", record.RepresentedCountry.Names)
	repl.Set("geoip2.representedcountry_type", record.RepresentedCountry.Type)
	for _, lc := range languageCodes {
		value := record.RepresentedCountry.Names[lc]
		repl.Set("geoip2.representedcountry_names_"+lc, value)
		if lc == "en" {
			repl.Set("geoip2.representedcountry_name", value)
		}
	}

	// Subdivisions
	// Make sure we have at least two subdivisions.
	// It's unfortunate we have to redefine the subdivision
	// struct here as the geoip2 package doesn't expose it.
	for len(record.Subdivisions) < subdivisionSize {
		record.Subdivisions = append(
			record.Subdivisions,
			struct {
				Names      map[string]string `maxminddb:"names"`
				IsoCode    string            `maxminddb:"iso_code"`
				GeoNameID  uint              `maxminddb:"geoname_id"`
				Confidence uint8             `maxminddb:"confidence"`
			}{},
		)
	}
	repl.Set("geoip2.subdivisions", record.Subdivisions)
	for index, subdivision := range record.Subdivisions {
		indexStr := strconv.Itoa(index + 1)
		repl.Set("geoip2.subdivisions_"+indexStr+"_confidence", subdivision.Confidence)
		repl.Set("geoip2.subdivisions_"+indexStr+"_geoname_id", subdivision.GeoNameID)
		repl.Set("geoip2.subdivisions_"+indexStr+"_iso_code", subdivision.IsoCode)
		repl.Set("geoip2.subdivisions_"+indexStr+"_names", subdivision.Names)
		for _, lc := range languageCodes {
			value := subdivision.Names[lc]
			repl.Set("geoip2.subdivisions_"+indexStr+"_names_"+lc, value)
			if lc == "en" {
				repl.Set("geoip2.subdivisions_"+indexStr+"_name", value)
			}
		}
	}

	// Traits
	repl.Set("geoip2.traits_autonomous_system_number", record.Traits.AutonomousSystemNumber)
	repl.Set(
		"geoip2.traits_autonomous_system_organization",
		record.Traits.AutonomousSystemOrganization,
	)
	repl.Set("geoip2.traits_connection_type", record.Traits.ConnectionType)
	repl.Set("geoip2.traits_domain", record.Traits.Domain)
	repl.Set("geoip2.traits_is_anonymous_proxy", record.Traits.IsAnonymousProxy)
	repl.Set("geoip2.traits_is_anycast", record.Traits.IsAnycast)
	repl.Set("geoip2.traits_is_legitimate_proxy", record.Traits.IsLegitimateProxy)
	repl.Set("geoip2.traits_is_satellite_provider", record.Traits.IsSatelliteProvider)
	repl.Set("geoip2.traits_isp", record.Traits.ISP)
	repl.Set("geoip2.traits_mobile_country_code", record.Traits.MobileCountryCode)
	repl.Set("geoip2.traits_mobile_network_code", record.Traits.MobileNetworkCode)
	repl.Set("geoip2.traits_organization", record.Traits.Organization)
	repl.Set("geoip2.traits_static_ip_score", record.Traits.StaticIPScore)
	repl.Set("geoip2.traits_user_type", record.Traits.UserType)
}
