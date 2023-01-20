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
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/maxmind/geoipupdate/v4/pkg/geoipupdate"
	"github.com/maxmind/geoipupdate/v4/pkg/geoipupdate/database"
	"github.com/oschwald/maxminddb-golang"
)

type GeoIP2State struct {
	mu                sync.Mutex
	DBHandler         *maxminddb.Reader
	AccountID         int    `json:"accountId,omitempty"`
	DatabaseDirectory string `json:"databaseDirectory,omitempty"`
	LicenseKey        string `json:"licenseKey,omitempty"`
	LockFile          string `json:"lockFile,omitempty"`
	EditionID         string `json:"editionID,omitempty"`
	UpdateUrl         string `json:"updateUrl,omitempty"`
	UpdateFrequency   int    `json:"updateFrequency,omitempty"`
	done              chan bool
}

var geoIP2State = GeoIP2State{}

func parseGeoip2(d *caddyfile.Dispenser, _ any) (any, error) {
	err := geoIP2State.UnmarshalCaddyfile(d)
	return nil, err
}

// for global
func (g *GeoIP2State) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	// func UnmarshalCaddyfile(d *caddyfile.Dispenser, _ any) (any, error) {
	// g := GeoIP2Config{}
	geoIP2State.mu.Lock()
	defer geoIP2State.mu.Unlock()

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
	caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("setup Config %v", g))

	if g.UpdateUrl == "" {
		g.UpdateUrl = "https://updates.maxmind.com"
	}

	if g.DatabaseDirectory == "" {
		g.DatabaseDirectory = "/tmp/"
	}
	if g.LockFile == "" {
		g.LockFile = "geoip2.lock"
	}

	if g.EditionID == "" {
		g.EditionID = "GeoLite2-City"
	}

	if g.UpdateFrequency > 0 && g.AccountID > 0 && g.LicenseKey != "" {
		g.runGeoIP2UpdateLoop()
	}

	return nil
}

func (g *GeoIP2State) runGeoIP2Update() {
	g.mu.Lock()
	defer g.mu.Unlock()
	config := geoipupdate.Config{
		AccountID:         g.AccountID,
		DatabaseDirectory: g.DatabaseDirectory,
		LicenseKey:        g.LicenseKey,
		LockFile:          g.LockFile,
		EditionIDs:        []string{g.EditionID},
		URL:               g.UpdateUrl,
	}
	caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("geoipupdate.Config %v", config))
	client := geoipupdate.NewClient(&config)
	dbReader := database.NewHTTPDatabaseReader(client, &config)
	editionID := config.EditionIDs[0]
	// for _, editionID := range config.EditionIDs {
	filename, err := geoipupdate.GetFilename(&config, editionID, client)

	caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("retrieving filename for %s", editionID))
	if err != nil {
		caddy.Log().Named("http.handlers.geoip2").Error(fmt.Sprintf("error retrieving filename for %s: %v", editionID, err))
	}
	filePath := filepath.Join(config.DatabaseDirectory, filename)
	if g.DBHandler == nil {
		g.DBHandler, _ = maxminddb.Open(filePath)
	}
	newFilePath := filePath + ".new"
	dbWriter, err := database.NewLocalFileDatabaseWriter(newFilePath, config.LockFile, config.Verbose)
	if err != nil {
		caddy.Log().Named("http.handlers.geoip2").Error(fmt.Sprintf("error creating database writer for %s: %v", editionID, err))
	}
	if err := dbReader.Get(dbWriter, editionID); err != nil {
		caddy.Log().Named("http.handlers.geoip2").Error(fmt.Sprintf("error creating database writer for %s: %v", editionID, err))
	}
	caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("filename for %s done", editionID))
	if _, err := os.Stat(newFilePath); errors.Is(err, fs.ErrNotExist) {
		caddy.Log().Named("http.handlers.geoip2").Error(fmt.Sprintf("downloadfile Error %v", err))
	} else {
		e := os.Rename(newFilePath, filePath)

		if e != nil {
			caddy.Log().Named("http.handlers.geoip2").Error(fmt.Sprintf("rename file  Error %v", err))
			return
		}

		newInstance, openerr := maxminddb.Open(filePath)
		if openerr != nil {
			caddy.Log().Named("http.handlers.geoip2").Error(fmt.Sprintf("open file  Error %s", filePath))
		}
		oldInstance := g.DBHandler
		g.DBHandler = newInstance
		if oldInstance != nil {
			oldInstance.Close()
		}
	}
}

func (g *GeoIP2State) runGeoIP2UpdateLoop() {
	go g.runGeoIP2Update()
	g.done = make(chan bool, 1)
	go func(t time.Duration) {
		tick := time.NewTicker(t).C
		for {
			select {
			// t has passed, so id can be destroyed
			case <-tick:

				caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("update tick %v", g))
				g.runGeoIP2Update()
				// We are finished destroying stuff
			case <-g.done:
				caddy.Log().Named("http.handlers.geoip2").Info(fmt.Sprintf("destroying"))
				return
			}
		}
	}(time.Second * time.Duration(g.UpdateFrequency))
}

func (g *GeoIP2State) Destruct() error {

	g.mu.Lock()
	defer g.mu.Unlock()

	// stop all background tasks
	if g.done != nil {
		close(g.done)
	}

	if g.DBHandler != nil {
		return g.DBHandler.Close()
	}

	return nil
}

var (
	_ caddyfile.Unmarshaler = (*GeoIP2State)(nil)
)
