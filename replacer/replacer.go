// Package replacer defines replacers for different geoip database types.
package replacer

import (
	"fmt"
	"net"

	"github.com/caddyserver/caddy/v2"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
)

// Replacer is a common interface for the various database types repacers.
type Replacer interface {
	Lookup(*caddy.Replacer, net.IP)
	Close() error
}

// New initializes a geoip replacer based on the provided database type.
func New(filePath string) (Replacer, error) {
	reader, err := maxminddb.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening geoip database reader: %w", err)
	}

	replacer, err := getReplacerForType(reader)
	return replacer, err
}

func getReplacerForType(reader *maxminddb.Reader) (Replacer, error) {
	switch reader.Metadata.DatabaseType {
	case
		"DBIP-City-Lite",
		"DBIP-Country",
		"DBIP-Country-Lite",
		"DBIP-ISP (compat=Enterprise)",
		"DBIP-Location (compat=City)",
		"DBIP-Location-ISP (compat=Enterprise)",
		"GeoIP2-City",
		"GeoIP2-City-Africa",
		"GeoIP2-City-Asia-Pacific",
		"GeoIP2-City-Europe",
		"GeoIP2-City-North-America",
		"GeoIP2-City-South-America",
		"GeoIP2-Country",
		"GeoIP2-Enterprise",
		"GeoIP2-Precision-City",
		"GeoLite2-City",
		"GeoLite2-Country":
		return &Enterprise{reader: reader}, nil
	case "DBIP-ASN-Lite (compat=GeoLite2-ASN)",
		"GeoIP2-ISP",
		"GeoIP2-Precision-ISP",
		"GeoLite2-ASN":
		return &ISP{reader: reader}, nil
	case "GeoIP2-Connection-Type":
		return &ConnectionType{reader: reader}, nil
	case "GeoIP2-Domain":
		return &Domain{reader: reader}, nil
	case "GeoIP2-Anonymous-IP":
		return &Anonymous{reader: reader}, nil
	default:
		return nil, fmt.Errorf("database type %q not supported", reader.Metadata.DatabaseType)
	}
}

// SetDefaultValues initializes the replacer with default values
// for geoip handler.
func SetDefaultValues(repl *caddy.Replacer) {
	repl.Set("geoip2.ip_address", "")

	SetAnonymous(repl, geoip2.AnonymousIP{})
	SetConnectionType(repl, geoip2.ConnectionType{})
	SetDomain(repl, geoip2.Domain{})
	SetISP(repl, geoip2.ISP{})
	SetEnterprise(repl, geoip2.Enterprise{})
}
