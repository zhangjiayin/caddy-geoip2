package replacer

import (
	"fmt"
	"net"

	"github.com/caddyserver/caddy/v2"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
	"go.uber.org/zap"
)

// ISP represents a geoip reader for ISP and ASN type databases.
type ISP struct {
	reader *maxminddb.Reader
}

// Lookup performs a database lookup on the provided clientIP and sets
// the related replacer variables.
func (r *ISP) Lookup(repl *caddy.Replacer, clientIP net.IP) {
	var record geoip2.ISP
	err := r.reader.Lookup(clientIP, &record)
	if err != nil {
		caddy.Log().Named("geoip2").
			Error(fmt.Sprintf(
				"looking up ISP record for IP %q: %+v",
				clientIP.String(),
				err,
			))
	}

	SetISP(repl, record)

	caddy.Log().Named("http.handlers.geoip2").
		Debug("Lookup ISP/ASN", zap.String("clientIP", clientIP.String()), zap.Any("record", record))
}

// Close closes the database reader.
func (r *ISP) Close() error {
	return r.reader.Close()
}

// SetISP sets values for possible replacer variables.
func SetISP(repl *caddy.Replacer, record geoip2.ISP) {
	repl.Set("geoip2.autonomous_system_number", record.AutonomousSystemNumber)
	repl.Set("geoip2.autonomous_system_organization", record.AutonomousSystemOrganization)
	repl.Set("geoip2.isp", record.ISP)
	repl.Set("geoip2.mobile_country_code", record.MobileCountryCode)
	repl.Set("geoip2.mobile_network_code", record.MobileNetworkCode)
	repl.Set("geoip2.organization", record.Organization)
}
