package replacer

import (
	"net"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/oschwald/geoip2-golang"
)

func TestDomainLookup(t *testing.T) {
	reader, err := New("test-data/test-data/GeoIP2-Domain-Test.mmdb")
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
	SetDomain(repl, geoip2.Domain{}) // set defaults.
	reader.Lookup(repl, net.ParseIP("71.160.223.137"))
	equal(t, repl, "geoip2.domain", "verizon.net")
}
