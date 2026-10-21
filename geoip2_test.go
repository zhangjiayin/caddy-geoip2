package geoip2

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestStruct(t *testing.T) {
	var state GeoIP2State
	if err := json.Unmarshal([]byte(`{"databaseDirectory":"dddd","accountId":333}`), &state); err != nil {
		t.Fatal(err)
	}
	if state.AccountID != 333 {
		t.Fatalf("AccountID = %d, want 333", state.AccountID)
	}
}

func TestServe(t *testing.T) {
	server := newGeoIPTestServer(t)
	server.load(t, 0, "")
	server.assertCountry(t, server.request(t))
}

// Every reload must retain working lookups, including requests served by an
// old connection while Caddy finishes shutting down its previous HTTP server.
func TestServeAfterReloads(t *testing.T) {
	server := newGeoIPTestServer(t)
	for i := 0; i < 30; i++ {
		server.load(t, i, "")
		server.assertCountry(t, server.request(t))
	}
}

func TestServeDuringReload(t *testing.T) {
	server := newGeoIPTestServer(t)
	gate := &reloadGate{entered: make(chan struct{}), release: make(chan struct{})}
	reloadGates.Store(server.address, gate)
	t.Cleanup(func() { reloadGates.Delete(server.address) })
	defer gate.resume()
	server.load(t, 0, server.address)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+server.address, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", "81.2.69.160")
	responses := make(chan *http.Response, 1)
	errors := make(chan error, 1)
	go func() {
		response, err := server.client.Do(req)
		if err != nil {
			errors <- err
			return
		}
		responses <- response
	}()
	select {
	case <-gate.entered:
	case err := <-errors:
		t.Fatal(err)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}

	// The old request is blocked before the GeoIP handler. Caddy stops its old
	// app before this new configuration finishes loading.
	server.load(t, 1, "")
	gate.resume()
	select {
	case response := <-responses:
		server.assertCountry(t, response)
		if got := response.Header.Get("X-Config"); got != "0" {
			t.Fatalf("old request used configuration %q, want 0", got)
		}
	case err := <-errors:
		t.Fatal(err)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
}
