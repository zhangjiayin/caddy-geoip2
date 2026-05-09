package geoip2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
)

// TestHelloName calls greetings.Hello with a name, checking
// for a valid return value.
func TestStruct(t *testing.T) {
	// geoIP2State.DatabaseDirectory = "asdfasdfsd"
	// v := caddyconfig.JSON(geoIP2State, nil)
	geoIP2State := GeoIP2State{}
	jsonStr := "{\"databaseDirectory\":\"dddd\",\"accountId\":333}"
	dec := json.NewDecoder(bytes.NewReader([]byte(jsonStr)))
	dec.DisallowUnknownFields()
	dec.Decode(&geoIP2State)
	fmt.Printf("%v\n", geoIP2State.AccountID)
}

func TestServe(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(fmt.Sprintf(cfg, caddytest.Default.AdminPort), "caddyfile")

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1:8080", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", "81.2.69.160")

	resp := tester.AssertResponseCode(req, http.StatusNoContent)

	want := "GB"
	if got := resp.Header.Get("x-geo-country"); got != want {
		t.Errorf("geoip2.country_code = %q, want %q", got, want)
	}
}

const cfg = `{
    admin :%d
    debug
    auto_https off
    order geoip2_vars first
    geoip2 {
        databaseDirectory "replacer/test-data/test-data"
        editionID        "GeoLite2-Country-Test"
    }
	servers {
        # Enable trusted_proxies to make Caddy populate client_ip variable from X-Forwarded-For header, in the case Caddy is behind a reverse proxy
        # See : https://caddyserver.com/docs/caddyfile/options#trusted-proxies
        trusted_proxies static private_ranges
    }
}

:8080 {
	geoip2_vars trusted_proxies
	header * x-geo-country "{geoip2.country_code}"
	respond 204
}`
