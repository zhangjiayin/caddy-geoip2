package replacer

import (
	"net"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/oschwald/geoip2-golang"
)

func TestAnonymousLookup(t *testing.T) {
	reader, err := New("test-data/test-data/GeoIP2-Anonymous-IP-Test.mmdb")
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
	SetAnonymous(repl, geoip2.AnonymousIP{}) // set defaults.
	reader.Lookup(repl, net.ParseIP("81.2.69.187"))
	equal(t, repl, "geoip2.is_anonymous", true)
	equal(t, repl, "geoip2.is_anonymous_vpn", true)
	equal(t, repl, "geoip2.is_hosting_provider", true)
	equal(t, repl, "geoip2.is_public_proxy", true)
	equal(t, repl, "geoip2.is_residential_proxy", true)
	equal(t, repl, "geoip2.is_tor_exit_node", true)
}
