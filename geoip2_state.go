package geoip2

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/maxmind/geoipupdate/v4/pkg/geoipupdate"
	"github.com/maxmind/geoipupdate/v4/pkg/geoipupdate/database"
	"github.com/zhangjiayin/caddy-geoip2/replacer"
	"go.uber.org/zap"
)

// GeoIP2State holds the configuration used
// to manage GeoIP database access and updates.
type GeoIP2State struct {
	// mutex synchronizes access to dbReaders.
	mutex         sync.RWMutex
	dbReaders     []replacer.Replacer
	readerCleanup runtime.Cleanup

	// done and wg are used to exit gracefully
	// and wait for any update to complete first.
	done     chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once

	// AccountID is your MaxMind account ID. This was formerly known as UserId.
	AccountID int `json:"accountId,omitempty"`
	// DatabaseDirectory specifies the directory where database files are stored.
	// Defaults to DATADIR.
	DatabaseDirectory string `json:"databaseDirectory,omitempty"`
	// LicenseKey is your case-sensitive MaxMind license key.
	LicenseKey string `json:"licenseKey,omitempty"`
	// LockFile specifies the path to the lock file used to ensure that only one
	// geoipupdate process runs at a time.
	// Note: Once created, this lock file is not removed from the filesystem.
	LockFile string `json:"lockFile,omitempty"`
	// EditionIDs is a comma-separated list of database edition IDs to update.
	// This allows simultaneous lookups across multiple databases.
	// Examples:
	// - "GeoLite2-City"
	// - "GeoLite2-Country,GeoLite2-ASN"
	// Note: The JSON tag uses "editionID" for backwards compatibility.
	EditionIDs []string `json:"editionID,omitempty"`
	// UpdateURL specifies the update server URL. Defaults to https://updates.maxmind.com.
	UpdateURL string `json:"updateUrl,omitempty"`
	// UpdateFrequency is the frequency in seconds at which the update runs.
	// Defaults to 0, which means the update runs only on start.
	UpdateFrequency int `json:"updateFrequency,omitempty"`
}

const (
	moduleName = "geoip2"
)

func init() {
	caddy.RegisterModule(&GeoIP2State{})
	httpcaddyfile.RegisterGlobalOption(
		moduleName,
		func(d *caddyfile.Dispenser, _ any) (any, error) {
			state := &GeoIP2State{}
			if err := state.UnmarshalCaddyfile(d); err != nil {
				return nil, err
			}
			return httpcaddyfile.App{
				Name:  moduleName,
				Value: caddyconfig.JSON(state, nil),
			}, nil
		},
	)
}

// CaddyModule implements caddy.Module.
func (g *GeoIP2State) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: moduleName,
		// Each config load needs its own instance: on reload Caddy starts the new
		// config before stopping the old one, and a shared instance let the old
		// config's Stop() close the readers the new config had just opened.
		New: func() caddy.Module { return new(GeoIP2State) },
	}
}

// Start implements caddy.App.
func (g *GeoIP2State) Start() error {
	caddy.Log().Named(moduleName).Debug("start")
	if err := g.ensureDirectories(); err != nil {
		return err
	}
	if g.AccountID > 0 && g.LicenseKey != "" {
		g.done = make(chan struct{})
		g.wg.Add(1)
		go g.runGeoIPUpdate()
	}
	return nil
}

// Stop implements caddy.App.
func (g *GeoIP2State) Stop() error {
	caddy.Log().Named(moduleName).Debug("stop")

	g.stopOnce.Do(func() {
		if g.done != nil {
			close(g.done)
		}
	})
	g.wg.Wait()

	// Caddy can still be draining old HTTP requests after Stop returns. Keep the
	// final readers available to their handlers; readerCleanup closes them once
	// this state is no longer reachable. Replaced readers are closed immediately.
	return nil
}

// Provision implements caddy.Provisioner.
func (g *GeoIP2State) Provision(_ caddy.Context) error {
	caddy.Log().Named(moduleName).Debug("provision")
	// Open existing local databases before any apps start serving requests.
	// Missing databases can still be downloaded asynchronously after Start.
	g.loadGeoIPReaders()
	return nil
}

// Validate implements caddy.Validator.
func (g *GeoIP2State) Validate() error {
	caddy.Log().Named(moduleName).Debug("validate")

	if g.DatabaseDirectory == "" || len(g.EditionIDs) == 0 {
		return fmt.Errorf("missing: DatabaseDirectory %q or EditionIDs %+v", g.DatabaseDirectory, g.EditionIDs)
	}
	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (g *GeoIP2State) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		var value string
		key := d.Val()
		if !d.Args(&value) {
			continue
		}
		switch key {
		case "accountId":
			accountID, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("accountID is not an integer: %w", err)
			}
			g.AccountID = accountID
		case "databaseDirectory":
			g.DatabaseDirectory = value
		case "licenseKey":
			g.LicenseKey = value
		case "lockFile":
			g.LockFile = value
		case "editionID":
			editionIDs := strings.Split(value, ",")
			for _, e := range editionIDs {
				g.EditionIDs = append(g.EditionIDs, strings.TrimSpace(e))
			}
		case "updateUrl":
			g.UpdateURL = value
		case "updateFrequency":
			updateFrequency, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("updateFrequency is not an integer: %w", err)
			}
			g.UpdateFrequency = updateFrequency
		}
	}

	if g.UpdateURL == "" {
		g.UpdateURL = "https://updates.maxmind.com"
	}
	if g.DatabaseDirectory == "" {
		g.DatabaseDirectory = "/tmp/"
	}
	if g.LockFile == "" {
		g.LockFile = "/tmp/geoip2.lock"
	}
	if len(g.EditionIDs) == 0 {
		g.EditionIDs = []string{"GeoLite2-City"}
	}
	caddy.Log().Named(moduleName).Debug("configuration loaded", zap.Any("config", g))
	return nil
}

func (g *GeoIP2State) lookup(repl *caddy.Replacer, clientIP net.IP) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	for _, r := range g.dbReaders {
		r.Lookup(repl, clientIP)
	}
	// The cleanup must not close the memory mappings during a lookup.
	runtime.KeepAlive(g)
}

func (g *GeoIP2State) loadGeoIPReaders() {
	caddy.Log().Named(moduleName).Debug("load geoip readers")
	var dbReaders []replacer.Replacer
	for _, editionID := range g.EditionIDs {
		filePath := filepath.Join(g.DatabaseDirectory, editionID+".mmdb")
		if _, err := os.Stat(filePath); errors.Is(err, fs.ErrNotExist) {
			caddy.Log().Named(moduleName).
				Error("missing geoip database file", zap.String("editionID", editionID))
			continue
		}
		dbReader, err := replacer.New(filePath)
		if err != nil {
			caddy.Log().Named(moduleName).
				Error("initializing geoip database reader", zap.String("editionID", editionID), zap.Error(err))
			continue
		}
		caddy.Log().Named(moduleName).
			Info("initialized geoip database reader", zap.String("editionID", editionID))
		dbReaders = append(dbReaders, dbReader)
	}

	g.replaceReaders(dbReaders)
}

func (g *GeoIP2State) replaceReaders(dbReaders []replacer.Replacer) {
	g.mutex.Lock()
	if len(dbReaders) == 0 && len(g.dbReaders) > 0 {
		// A failed reload must not discard databases that are still usable.
		g.mutex.Unlock()
		return
	}
	previous := g.dbReaders
	g.readerCleanup.Stop()
	g.dbReaders = dbReaders
	if len(dbReaders) > 0 {
		// Neither the cleanup function nor its argument may retain g.
		g.readerCleanup = runtime.AddCleanup(g, closeGeoIPReaders, dbReaders)
	}
	g.mutex.Unlock()
	closeGeoIPReaders(previous)
	// Keep g reachable until its previous cleanup has been canceled.
	runtime.KeepAlive(g)
}

func closeGeoIPReaders(readers []replacer.Replacer) {
	for _, reader := range readers {
		if err := reader.Close(); err != nil {
			caddy.Log().Named(moduleName).Warn("closing geoip database reader", zap.Error(err))
		}
	}
}

func (g *GeoIP2State) runGeoIPUpdate() {
	defer g.wg.Done()
	select {
	case <-g.done:
		return
	default:
	}

	config := geoipupdate.Config{
		AccountID:         g.AccountID,
		DatabaseDirectory: g.DatabaseDirectory,
		LicenseKey:        g.LicenseKey,
		LockFile:          g.LockFile,
		EditionIDs:        g.EditionIDs,
		URL:               g.UpdateURL,
	}
	// We currently have to use an older version of the geoipupdate
	// library because the newer client does not provide a convenient way
	// to write database files to disk. We should keep an eye on future
	// updates to that package to determine when an upgrade becomes feasible.
	client := geoipupdate.NewClient(&config)
	dbReader := database.NewHTTPDatabaseReader(client, &config)

	update := func() {
		caddy.Log().Named(moduleName).Debug("update geoip databases")
		for _, editionID := range g.EditionIDs {
			filePath := filepath.Join(g.DatabaseDirectory, editionID+".mmdb")
			dbWriter, err := database.NewLocalFileDatabaseWriter(
				filePath,
				config.LockFile,
				config.Verbose,
			)
			if err != nil {
				caddy.Log().Named(moduleName).
					Error("creating database writer", zap.String("editionID", editionID), zap.Error(err))
				continue
			}
			if err := dbReader.Get(dbWriter, editionID); err != nil {
				caddy.Log().Named(moduleName).
					Error("downloading new database file", zap.String("editionID", editionID), zap.Error(err))
				continue
			}
			caddy.Log().Named(moduleName).
				Info("updated database file", zap.String("editionID", editionID))
		}
		g.loadGeoIPReaders()
	}

	// run the update at least once to download database files
	// the first time.
	update()

	if g.UpdateFrequency != 0 {
		ticker := time.NewTicker(time.Second * time.Duration(g.UpdateFrequency))
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				update()
			case <-g.done:
				return
			}
		}
	}
}

func (g *GeoIP2State) hasDBReaders() bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return len(g.dbReaders) > 0
}

func (g *GeoIP2State) ensureDirectories() error {
	// Create the database directory if needed
	if err := mkdirIfMissing(g.DatabaseDirectory); err != nil {
		return fmt.Errorf("ensuring database directory %q: %w", g.DatabaseDirectory, err)
	}

	// Create the lock file directory if needed
	lockFileDir := filepath.Dir(g.LockFile)

	if err := mkdirIfMissing(lockFileDir); err != nil {
		return fmt.Errorf("ensuring lock file directory %q: %w", lockFileDir, err)
	}

	return nil
}

func mkdirIfMissing(path string) error {
	if path == "" {
		// If the folder path is empty, we can't create it
		return nil
	}

	return os.MkdirAll(path, 0o700)
}

var (
	_ caddyfile.Unmarshaler = (*GeoIP2State)(nil)
	_ caddy.Module          = (*GeoIP2State)(nil)
	_ caddy.Provisioner     = (*GeoIP2State)(nil)
	_ caddy.Validator       = (*GeoIP2State)(nil)
	_ caddy.App             = (*GeoIP2State)(nil)
)
