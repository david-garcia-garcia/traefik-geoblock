package maxmind

const (
	// DefaultSeedFileName is MaxMind's official dummy Country fixture under seeds/.
	DefaultSeedFileName = "GeoIP2-Country-Test.mmdb"

	// DefaultGeoliteURL is the unofficial P3TERX Country MMDB on the download branch.
	// Official GeoLite needs accountId:licenseKey; this tree does not embed a live GeoLite file.
	DefaultGeoliteURL = "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb"
)
