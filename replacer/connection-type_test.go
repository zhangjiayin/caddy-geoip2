package replacer

import (
	"net"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/oschwald/geoip2-golang"
)

func TestConnectionTypeLookup(t *testing.T) {
	reader, err := New("test-data/test-data/GeoIP2-Connection-Type-Test.mmdb")
	if err != nil {
		t.Fatalf("initializing db reader: %+v", err)
	}

	t.Cleanup(func() {
		err := reader.Close()
		if err != nil {
			t.Fatalf("closing db reader: %+v", err)
		}
	})

	repl := caddy.NewEmptyReplacer()
	SetConnectionType(repl, geoip2.ConnectionType{}) // set defaults.
	reader.Lookup(repl, net.ParseIP("67.43.156.42"))
	equal(t, repl, "geoip2.connection_type", "Cellular")
}
