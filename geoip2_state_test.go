package geoip2

import (
	"net"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/zhangjiayin/caddy-geoip2/replacer"
)

func TestModuleConstructorsReturnDistinctInstances(t *testing.T) {
	for _, name := range []string{moduleName, "http.handlers.geoip2"} {
		t.Run(name, func(t *testing.T) {
			module, err := caddy.GetModule(name)
			if err != nil {
				t.Fatal(err)
			}
			if module.New() == module.New() {
				t.Fatal("module constructor reused an instance")
			}
		})
	}
}

func TestProvisionLoadsReadersAndFailedRefreshPreservesThem(t *testing.T) {
	state := &GeoIP2State{
		DatabaseDirectory: "replacer/test-data/test-data",
		EditionIDs:        []string{"GeoLite2-Country-Test"},
	}
	t.Cleanup(func() { disposeTestReaders(state) })
	if err := state.Provision(caddy.Context{}); err != nil {
		t.Fatal(err)
	}
	assertStateCountry(t, state)
	state.EditionIDs = []string{"missing-database"}
	state.loadGeoIPReaders()
	assertStateCountry(t, state)
}

func TestReadersAreClosedWhenReplaced(t *testing.T) {
	previous := &trackedReader{closed: make(chan struct{})}
	current := &trackedReader{closed: make(chan struct{})}
	state := new(GeoIP2State)
	t.Cleanup(func() { disposeTestReaders(state) })
	state.replaceReaders([]replacer.Replacer{previous})
	state.replaceReaders([]replacer.Replacer{current})
	if got := previous.closes.Load(); got != 1 {
		t.Fatalf("replaced reader closed %d times, want 1", got)
	}
	if got := current.closes.Load(); got != 0 {
		t.Fatalf("current reader closed %d times, want 0", got)
	}
	runtime.KeepAlive(state)
}

func TestStoppedReadersLiveUntilStateIsUnreachable(t *testing.T) {
	closed := make(chan struct{})
	func() {
		state := new(GeoIP2State)
		state.replaceReaders([]replacer.Replacer{&trackedReader{closed: closed}})
		if err := state.Stop(); err != nil {
			t.Fatal(err)
		}
		runtime.GC()
		select {
		case <-closed:
			t.Fatal("readers closed while old handlers can still reach the state")
		default:
		}
		runtime.KeepAlive(state)
	}()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		runtime.GC()
		select {
		case <-closed:
			return
		case <-deadline.C:
			t.Fatal("unreachable state did not release its readers")
		case <-tick.C:
		}
	}
}

func TestStopWaitsForUpdaterAndIsIdempotent(t *testing.T) {
	state := &GeoIP2State{done: make(chan struct{})}
	state.wg.Add(1)
	stopped := make(chan struct{})
	go func() {
		_ = state.Stop()
		close(stopped)
	}()
	<-state.done
	select {
	case <-stopped:
		t.Fatal("Stop returned before the updater finished")
	default:
	}
	state.wg.Done()
	<-stopped
	if err := state.Stop(); err != nil {
		t.Fatal(err)
	}
}

func assertStateCountry(t *testing.T, state *GeoIP2State) {
	t.Helper()
	repl := caddy.NewReplacer()
	state.lookup(repl, net.ParseIP("81.2.69.160"))
	if country, _ := repl.Get("geoip2.country_code"); country != "GB" {
		t.Fatalf("geoip2.country_code = %v, want GB", country)
	}
}

func disposeTestReaders(state *GeoIP2State) {
	state.readerCleanup.Stop()
	closeGeoIPReaders(state.dbReaders)
	runtime.KeepAlive(state)
}

type trackedReader struct {
	closes atomic.Int32
	closed chan struct{}
}

func (*trackedReader) Lookup(*caddy.Replacer, net.IP) {}

func (r *trackedReader) Close() error {
	if r.closes.Add(1) == 1 {
		close(r.closed)
	}
	return nil
}
