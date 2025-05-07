package replacer

import (
	"fmt"
	"net"

	"github.com/caddyserver/caddy/v2"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
)

// ConnectionType represents a geoip reader for ConnectionType databases.
type ConnectionType struct {
	reader *maxminddb.Reader
}

// Lookup performs a database lookup on the provided clientIP and sets
// the related replacer variables.
func (r *ConnectionType) Lookup(repl *caddy.Replacer, clientIP net.IP) {
	var record geoip2.ConnectionType
	err := r.reader.Lookup(clientIP, &record)
	if err != nil {
		caddy.Log().Named("geoip2").
			Error(fmt.Sprintf(
				"looking up ConnectionType record for IP %q: %+v",
				clientIP.String(),
				err,
			))
	}

	SetConnectionType(repl, record)

	caddy.Log().Named("http.handlers.geoip2").
		Debug(fmt.Sprintf("Lookup Connection-Type: %+v - %+v", record, clientIP))
}

// Close closes the database reader.
func (r *ConnectionType) Close() error {
	return r.reader.Close()
}

// SetConnectionType sets values for possible replacer variables.
func SetConnectionType(repl *caddy.Replacer, record geoip2.ConnectionType) {
	repl.Set("geoip2.connection_type", record.ConnectionType)
}
