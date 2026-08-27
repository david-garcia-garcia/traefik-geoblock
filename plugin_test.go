package traefik_geoblock

import (
	"net/http"
	"os"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
)

const (
	pluginName     = "geoblock"
	dbFilePath     = "./IP2LOCATION-LITE-DB1.IPV6.BIN"
	db8FilePath    = "./testdata/IP2LOCATION-DB8.BIN"
	ipinfoFilePath  = "./ipinfo_lite.mmdb"
	maxmindFilePath = "./GeoIP2-Country-Test.mmdb"
)

var fullEnrichHeaders = map[string]string{
	"X-Geo-Country": dbprovider.MetaCountry,
	"X-Geo-Region":  dbprovider.MetaRegion,
	"X-Geo-City":    dbprovider.MetaCity,
	"X-Geo-Isp":     dbprovider.MetaIsp,
	"X-Geo-Domain":  dbprovider.MetaDomain,
	"X-Geo-Asn":     dbprovider.MetaAsn,
}

func requireDB8(tb testing.TB) string {
	tb.Helper()
	if _, err := os.Stat(db8FilePath); err != nil {
		tb.Skip("paid DB8 BIN not present; place testdata/IP2LOCATION-DB8.BIN")
	}
	return db8FilePath
}

func requireASN(tb testing.TB) string {
	tb.Helper()
	if p := os.Getenv("IP2LOCATION_ASN_BIN"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, candidate := range []string{
		"./IP2LOCATION-LITE-ASN.IPV6.BIN",
		"./testdata/IP2LOCATION-LITE-ASN.IPV6.BIN",
		`D:\IP2LOCATION-LITE-ASN.IPV6.BIN`,
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	tb.Skip("ASN LITE BIN not present; set IP2LOCATION_ASN_BIN or place IP2LOCATION-LITE-ASN.IPV6.BIN")
	return ""
}

type noopHandler struct{}

func (n noopHandler) ServeHTTP(rw http.ResponseWriter, _ *http.Request) {
	rw.WriteHeader(http.StatusTeapot)
}
