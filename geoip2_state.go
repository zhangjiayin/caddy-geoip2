package geoip2

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/maxmind/geoipupdate/v4/pkg/geoipupdate"
	"github.com/maxmind/geoipupdate/v4/pkg/geoipupdate/database"
	"github.com/oschwald/maxminddb-golang"
)

// geoip2 is global caddy app with http.handlers.geoip2
// it update geoip2 data automatically by the params
type GeoIP2State struct {
	DBHandler *maxminddb.Reader `json:"-"`
	mutex     *sync.Mutex       `json:"-"`
	// Your MaxMind account ID. This was formerly known as UserId.
	AccountID int `json:"accountId,omitempty"`
	// The directory to store the database files. Defaults to DATADIR
	DatabaseDirectory string `json:"databaseDirectory,omitempty"`
	// Your case-sensitive MaxMind license key.
	LicenseKey string `json:"licenseKey,omitempty"`
	// The lock file to use. This ensures only one geoipupdate process can run at a
	// time.
	// Note: Once created, this lockfile is not removed from the filesystem.
	LockFile string `json:"lockFile,omitempty"`
	//Enter the edition IDs of the databases you would like to update.
	//Should be  GeoLite2-City
	EditionID string `json:"editionID,omitempty"`
	//update url to use. Defaults to https://updates.maxmind.com
	UpdateUrl string `json:"updateUrl,omitempty"`
	// The Frequency in seconds to run update. Default to 0, only update On Start
	UpdateFrequency int       `json:"updateFrequency,omitempty"`
	done            chan bool `json:"-"`
}

const (
	moduleName = "geoip2"
)

func init() {
	caddy.RegisterModule(GeoIP2State{})
	httpcaddyfile.RegisterGlobalOption("geoip2", parseGeoip2)
}

func (GeoIP2State) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "geoip2",
		New: func() caddy.Module { return new(GeoIP2State) },
	}
}

func parseGeoip2(d *caddyfile.Dispenser, _ any) (any, error) {
	state := GeoIP2State{}
	err := state.UnmarshalCaddyfile(d)
	return httpcaddyfile.App{
		Name:  "geoip2",
		Value: caddyconfig.JSON(state, nil),
	}, err
}
func (g *GeoIP2State) Start() error {
	if g.mutex == nil {
		g.mutex = &sync.Mutex{}
	}
	caddy.Log().Named("geoip2").Info(fmt.Sprintf("Start"))
	if g.DatabaseDirectory != "" && g.EditionID != "" {
		go g.runGeoIP2Update()
	}
	if g.UpdateFrequency > 0 && g.AccountID > 0 && g.LicenseKey != "" {
		g.runGeoIP2UpdateLoop()
	}
	return nil
}
func (g *GeoIP2State) Stop() error {
	if g.done != nil {
		g.done <- true
		caddy.Log().Named("geoip2").Debug(fmt.Sprintf("Send true to done chan"))
	}
	if g.DBHandler != nil {
		g.DBHandler.Close()
		caddy.Log().Named("geoip2").Debug(fmt.Sprintf("Close DBHandler"))
	}
	caddy.Log().Named("geoip2").Info(fmt.Sprintf("Stop"))

	return nil
}

// for global
func (g *GeoIP2State) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	if g.mutex == nil {
		g.mutex = &sync.Mutex{}
	}
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for d.Next() {
		var value string
		key := d.Val()
		if !d.Args(&value) {
			continue
		}
		switch key {
		case "accountId":
			AccountID, err := strconv.Atoi(value)
			if err == nil {
				g.AccountID = AccountID
			}
			break
		case "databaseDirectory":
			g.DatabaseDirectory = value
			break
		case "licenseKey":
			g.LicenseKey = value
			break
		case "lockFile":
			g.LockFile = value
			break
		case "editionID":
			g.EditionID = value
			break
		case "updateUrl":
			g.UpdateUrl = value
			break
		case "updateFrequency":
			UpdateFrequency, err := strconv.Atoi(value)
			if err == nil {
				g.UpdateFrequency = UpdateFrequency
			}
			break
		}
	}
	caddy.Log().Named("geoip2").Info(fmt.Sprintf("setup Config %v", g))

	if g.UpdateUrl == "" {
		g.UpdateUrl = "https://updates.maxmind.com"
	}

	if g.DatabaseDirectory == "" {
		g.DatabaseDirectory = "/tmp/"
	}
	if g.LockFile == "" {
		g.LockFile = "/tmp/geoip2.lock"
	}

	if g.EditionID == "" {
		g.EditionID = "GeoLite2-City"
	}

	return nil
}

func (g *GeoIP2State) runGeoIP2Update() {
	if g.mutex == nil {
		g.mutex = &sync.Mutex{}
	}
	g.mutex.Lock()
	defer g.mutex.Unlock()
	config := geoipupdate.Config{
		AccountID:         g.AccountID,
		DatabaseDirectory: g.DatabaseDirectory,
		LicenseKey:        g.LicenseKey,
		LockFile:          g.LockFile,
		EditionIDs:        []string{g.EditionID},
		URL:               g.UpdateUrl,
	}
	if g.DatabaseDirectory == "" || g.EditionID == "" {
		caddy.Log().Named("geoip2").Error(fmt.Sprintf("database is not loaded DatabaseDirectory %s   EditionID %s", g.DatabaseDirectory, g.EditionID))
		return
	}
	caddy.Log().Named("geoip2").Info(fmt.Sprintf("geoipupdate.Config %v", config))
	client := geoipupdate.NewClient(&config)
	dbReader := database.NewHTTPDatabaseReader(client, &config)
	editionID := config.EditionIDs[0]
	// for _, editionID := range config.EditionIDs {
	filename, err := geoipupdate.GetFilename(&config, editionID, client)

	caddy.Log().Named("geoip2").Info(fmt.Sprintf("retrieving filename for %s", editionID))
	if err != nil {
		caddy.Log().Named("geoip2").Error(fmt.Sprintf("error retrieving filename for %s: %v", editionID, err))
	}
	filePath := filepath.Join(config.DatabaseDirectory, filename)
	if g.DBHandler == nil {
		g.DBHandler, _ = maxminddb.Open(filePath)
	}
	if config.AccountID <= 0 || config.LicenseKey == "" || g.UpdateFrequency <= 0 {
		caddy.Log().Named("geoip2").Info(fmt.Sprintf("auto update is not enabled AccountID %d LicenseKey %s UpdateFrequency %d", config.AccountID, config.LicenseKey, g.UpdateFrequency))
		return
	}
	newFilePath := filePath + ".new"
	dbWriter, err := database.NewLocalFileDatabaseWriter(newFilePath, config.LockFile, config.Verbose)
	if err != nil {
		caddy.Log().Named("geoip2").Error(fmt.Sprintf("error creating database writer for %s: %v", editionID, err))
	}
	if err := dbReader.Get(dbWriter, editionID); err != nil {
		caddy.Log().Named("geoip2").Error(fmt.Sprintf("error creating database writer for %s: %v", editionID, err))
	}
	caddy.Log().Named("geoip2").Info(fmt.Sprintf("filename for %s done", editionID))
	if _, err := os.Stat(newFilePath); errors.Is(err, fs.ErrNotExist) {
		caddy.Log().Named("geoip2").Error(fmt.Sprintf("downloadfile Error %v", err))
	} else {

		caddy.Log().Named("geoip2").Debug(fmt.Sprintf("downloadfile Error %v", err))
		e := os.Rename(newFilePath, filePath)
		caddy.Log().Named("geoip2").Debug(fmt.Sprintf("rename  %s %s %v", newFilePath, filePath, e))
		if e != nil {
			caddy.Log().Named("geoip2").Error(fmt.Sprintf("rename file  Error %v", err))
			return
		}

		newInstance, openerr := maxminddb.Open(filePath)
		if openerr != nil {
			caddy.Log().Named("geoip2").Error(fmt.Sprintf("open file  Error %s", filePath))
		}
		oldInstance := g.DBHandler
		g.DBHandler = newInstance
		if oldInstance != nil {
			oldInstance.Close()
		}
	}
}

func (g *GeoIP2State) runGeoIP2UpdateLoop() {
	g.done = make(chan bool, 1)
	go func(t time.Duration) {
		tick := time.NewTicker(t).C
		for {
			select {
			// t has passed, so id can be destroyed
			case <-tick:

				caddy.Log().Named("geoip2").Info(fmt.Sprintf("update tick %v", g))
				g.runGeoIP2Update()
				// We are finished destroying stuff
			case <-g.done:
				caddy.Log().Named("geoip2").Info(fmt.Sprintf("destroying"))
				return
			}
		}
	}(time.Second * time.Duration(g.UpdateFrequency))
}

func (g *GeoIP2State) Destruct() error {
	if g.mutex == nil {
		g.mutex = &sync.Mutex{}
	}
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// stop all background tasks
	if g.done != nil {
		close(g.done)
	}

	if g.DBHandler != nil {
		return g.DBHandler.Close()
	}

	return nil
}

func (g *GeoIP2State) Provision(ctx caddy.Context) error {
	caddy.Log().Named("geoip2").Info(fmt.Sprintf("Provision"))
	return nil
}
func (g GeoIP2State) Validate() error {
	caddy.Log().Named("geoip2").Info(fmt.Sprintf("Validate"))

	if g.DatabaseDirectory == "" || g.EditionID == "" {
		return fmt.Errorf("DatabaseDirectory %s EditionID %s is not avalidate", g.DatabaseDirectory, g.EditionID)
	}

	if g.AccountID <= 0 || g.LicenseKey == "" || g.UpdateFrequency <= 0 {
		filePath := filepath.Join(g.DatabaseDirectory, g.EditionID+".mmdb")
		if _, err := os.Stat(filePath); errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("DatabaseDirectory %s EditionID %s file not found", g.DatabaseDirectory, g.EditionID)
		}
	}
	return nil

}

var (
	_ caddyfile.Unmarshaler = (*GeoIP2State)(nil)
	_ caddy.Module          = (*GeoIP2State)(nil)
	_ caddy.Provisioner     = (*GeoIP2State)(nil)
	_ caddy.Validator       = (*GeoIP2State)(nil)
	_ caddy.App             = (*GeoIP2State)(nil)
)
