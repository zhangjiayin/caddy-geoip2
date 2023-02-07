package geoip2

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestHelloName calls greetings.Hello with a name, checking
// for a valid return value.
func TestStruct(t *testing.T) {
	// geoIP2State.DatabaseDirectory = "asdfasdfsd"
	// v := caddyconfig.JSON(geoIP2State, nil)
	jsonStr := "{\"databaseDirectory\":\"dddd\",\"accountId\":333}"
	dec := json.NewDecoder(bytes.NewReader([]byte(jsonStr)))
	dec.DisallowUnknownFields()
}
