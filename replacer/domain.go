package replacer

import (
	"fmt"
	"net"

	"github.com/caddyserver/caddy/v2"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
	"go.uber.org/zap"
)

// Domain represents a geoip reader for Domain type databases.
type Domain struct {
	reader *maxminddb.Reader
}

// Lookup performs a database lookup on the provided clientIP and sets
// the related replacer variables.
func (r *Domain) Lookup(repl *caddy.Replacer, clientIP net.IP) {
	var record geoip2.Domain
	err := r.reader.Lookup(clientIP, &record)
	if err != nil {
		caddy.Log().Named("geoip2").
			Error(fmt.Sprintf(
				"looking up Domain record for IP %q: %+v",
				clientIP.String(),
				err,
			))
	}

	SetDomain(repl, record)

	caddy.Log().Named("http.handlers.geoip2").
		Debug("Lookup Domain", zap.String("clientIP", clientIP.String()), zap.Any("record", record))
}

// Close closes the database reader.
func (r *Domain) Close() error {
	return r.reader.Close()
}

// SetDomain sets values for possible replacer variables.
func SetDomain(repl *caddy.Replacer, record geoip2.Domain) {
	repl.Set("geoip2.domain", record.Domain)
}
