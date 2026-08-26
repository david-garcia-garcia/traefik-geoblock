package dbprovider

// Provider opens a geo database, looks up a country, and owns auto-update.
type Provider interface {
	LookupCountry(ip string) (string, error)
	Close() error
}
