package replacer

import (
	"net"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/oschwald/geoip2-golang"
)

func TestISPLookup(t *testing.T) {
	reader, err := New("test-data/test-data/GeoIP2-ISP-Test.mmdb")
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
	SetISP(repl, geoip2.ISP{}) // set defaults.
	reader.Lookup(repl, net.ParseIP("1.130.5.12"))
	equal(t, repl, "geoip2.autonomous_system_number", uint(1221))
	equal(t, repl, "geoip2.autonomous_system_organization", "Telstra Pty Ltd")
	equal(t, repl, "geoip2.isp", "Telstra Internet")
	equal(t, repl, "geoip2.mobile_country_code", "")
	equal(t, repl, "geoip2.mobile_network_code", "")
	equal(t, repl, "geoip2.organization", "Telstra Internet")
}

func TestASNLookup(t *testing.T) {
	reader, err := New("test-data/test-data/GeoLite2-ASN-Test.mmdb")
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
	SetISP(repl, geoip2.ISP{}) // set defaults.
	reader.Lookup(repl, net.ParseIP("1.130.5.12"))
	equal(t, repl, "geoip2.autonomous_system_number", uint(1221))
	equal(t, repl, "geoip2.autonomous_system_organization", "Telstra Pty Ltd")
	equal(t, repl, "geoip2.isp", "")
	equal(t, repl, "geoip2.mobile_country_code", "")
	equal(t, repl, "geoip2.mobile_network_code", "")
	equal(t, repl, "geoip2.organization", "")
}
