package geoip2

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/headers"
)

type geoIPTestServer struct {
	address string
	client  *http.Client
}

func newGeoIPTestServer(t *testing.T) *geoIPTestServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{}, Timeout: 5 * time.Second}
	t.Cleanup(func() {
		client.CloseIdleConnections()
		if err := caddy.Stop(); err != nil {
			t.Error(err)
		}
	})
	return &geoIPTestServer{address: address, client: client}
}

func (s *geoIPTestServer) load(t *testing.T, version int, gate string) {
	t.Helper()
	handlers := []any{}
	if gate != "" {
		handlers = append(handlers, map[string]any{"handler": "geoip2_reload_gate", "id": gate})
	}
	handlers = append(handlers,
		map[string]any{"handler": "geoip2", "enable": "trusted_proxies"},
		map[string]any{"handler": "headers", "response": map[string]any{"set": map[string]any{
			"X-Geo-Country": []string{"{geoip2.country_code}"},
			"X-Config":      []string{strconv.Itoa(version)},
		}}},
		map[string]any{"handler": "static_response", "status_code": 204},
	)
	config := map[string]any{
		"admin":   map[string]any{"disabled": true, "config": map[string]any{"persist": false}},
		"logging": map[string]any{"logs": map[string]any{"default": map[string]any{"level": "ERROR"}}},
		"apps": map[string]any{
			"geoip2": map[string]any{
				"databaseDirectory": "replacer/test-data/test-data",
				"editionID":         []string{"GeoLite2-Country-Test"},
			},
			"http": map[string]any{"servers": map[string]any{"test": map[string]any{
				"listen":          []string{s.address},
				"automatic_https": map[string]any{"disable": true},
				"trusted_proxies": map[string]any{"source": "static", "ranges": []string{"127.0.0.0/8"}},
				"routes":          []any{map[string]any{"handle": handlers}},
			}}},
		},
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := caddy.Load(encoded, true); err != nil {
		t.Fatal(err)
	}
}

func (s *geoIPTestServer) request(t *testing.T) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+s.address, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", "81.2.69.160")
	response, err := s.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func (s *geoIPTestServer) assertCountry(t *testing.T, response *http.Response) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.StatusCode)
	}
	if got := response.Header.Get("X-Geo-Country"); got != "GB" {
		t.Fatalf("config %s: geoip2.country_code = %q, want GB", response.Header.Get("X-Config"), got)
	}
}

type reloadGate struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (g *reloadGate) resume() { g.once.Do(func() { close(g.release) }) }

var reloadGates sync.Map

type reloadGateHandler struct {
	ID string `json:"id"`
}

func init() { caddy.RegisterModule(new(reloadGateHandler)) }

func (reloadGateHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.geoip2_reload_gate",
		New: func() caddy.Module { return new(reloadGateHandler) },
	}
}

func (h *reloadGateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	value, _ := reloadGates.Load(h.ID)
	gate := value.(*reloadGate)
	close(gate.entered)
	select {
	case <-gate.release:
		return next.ServeHTTP(w, r)
	case <-r.Context().Done():
		return r.Context().Err()
	}
}
