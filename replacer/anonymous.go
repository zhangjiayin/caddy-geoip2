package replacer

import (
	"fmt"
	"net"

	"github.com/caddyserver/caddy/v2"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
	"go.uber.org/zap"
)

// Anonymous represents a geoip reader for Anonymous databases.
type Anonymous struct {
	reader *maxminddb.Reader
}

// Lookup performs a database lookup on the provided clientIP and sets
// the related replacer variables.
func (r *Anonymous) Lookup(repl *caddy.Replacer, clientIP net.IP) {
	var record geoip2.AnonymousIP
	err := r.reader.Lookup(clientIP, &record)
	if err != nil {
		caddy.Log().Named("geoip2").
			Error(fmt.Sprintf(
				"looking up Anonymous record for IP %q: %+v",
				clientIP.String(),
				err,
			))
	}

	SetAnonymous(repl, record)

	caddy.Log().Named("http.handlers.geoip2").
		Debug("Lookup Anonymous", zap.String("clientIP", clientIP.String()), zap.Any("record", record))
}

// Close closes the database reader.
func (r *Anonymous) Close() error {
	return r.reader.Close()
}

// SetAnonymous sets values for possible replacer variables.
func SetAnonymous(repl *caddy.Replacer, record geoip2.AnonymousIP) {
	repl.Set("geoip2.is_anonymous", record.IsAnonymous)
	repl.Set("geoip2.is_anonymous_vpn", record.IsAnonymousVPN)
	repl.Set("geoip2.is_hosting_provider", record.IsHostingProvider)
	repl.Set("geoip2.is_public_proxy", record.IsPublicProxy)
	repl.Set("geoip2.is_residential_proxy", record.IsResidentialProxy)
	repl.Set("geoip2.is_tor_exit_node", record.IsTorExitNode)
}
