package ip2location

import "github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"

const (
	defaultGeoFileName = "IP2LOCATION-LITE-DB1.IPV6.BIN"
	defaultASNFileName = "IP2LOCATION-LITE-ASN.IPV6.BIN"
)

func defaultFileNameForSlot(slot string) string {
	if slot == dbutils.SlotASN {
		return defaultASNFileName
	}
	return defaultGeoFileName
}
