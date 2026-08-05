package geoip

import (
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

const defaultCityDatabasePath = "/opt/slan/data/GeoLite2-City.mmdb"

type Location struct {
	CountryCode string
	CityCode    string
}

type cityRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	RegisteredCountry struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"registered_country"`
	City struct {
		GeoNameID uint `maxminddb:"geoname_id"`
	} `maxminddb:"city"`
}

var (
	defaultOnce    sync.Once
	defaultLocator *Locator
)

type Locator struct {
	reader *maxminddb.Reader
}

func DefaultLookup(value string) (Location, bool) {
	defaultOnce.Do(func() {
		path := strings.TrimSpace(os.Getenv("SLAN_GEOIP_CITY_DB"))
		if path == "" {
			path = defaultCityDatabasePath
		}
		reader, err := maxminddb.Open(path)
		if err == nil {
			defaultLocator = &Locator{reader: reader}
		}
	})
	if defaultLocator == nil {
		return Location{}, false
	}
	return defaultLocator.Lookup(value)
}

func (l *Locator) Lookup(value string) (Location, bool) {
	if l == nil || l.reader == nil {
		return Location{}, false
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return Location{}, false
	}
	var record cityRecord
	if err := l.reader.Lookup(ip.Unmap()).Decode(&record); err != nil {
		return Location{}, false
	}
	countryCode := strings.ToUpper(strings.TrimSpace(record.Country.ISOCode))
	if countryCode == "" {
		countryCode = strings.ToUpper(strings.TrimSpace(record.RegisteredCountry.ISOCode))
	}
	if countryCode == "" {
		return Location{}, false
	}
	cityCode := ""
	if record.City.GeoNameID > 0 {
		cityCode = strconv.FormatUint(uint64(record.City.GeoNameID), 10)
	}
	return Location{CountryCode: countryCode, CityCode: cityCode}, true
}
