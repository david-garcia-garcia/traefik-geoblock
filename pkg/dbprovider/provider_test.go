package dbprovider

import "testing"

func TestRecordFieldAndKnownMetaKey(t *testing.T) {
	rec := Record{
		Country: "US", CountryName: "United States", Continent: "North America", ContinentCode: "NA",
		Region: "California", City: "Mountain View", Isp: "Google LLC", Domain: "google.com", Asn: "15169",
	}
	cases := []struct {
		key, want string
		known     bool
	}{
		{"country", "US", true},
		{"country_name", "United States", true},
		{"continent", "North America", true},
		{"continent_code", "NA", true},
		{"region", "California", true},
		{"city", "Mountain View", true},
		{"isp", "Google LLC", true},
		{"domain", "google.com", true},
		{"asn", "15169", true},
		{"ASN", "15169", true},
		{"as", "", false},
	}
	for _, tc := range cases {
		if got := rec.Field(tc.key); got != tc.want {
			t.Errorf("Field(%q)=%q, want %q", tc.key, got, tc.want)
		}
		if got := KnownMetaKey(tc.key); got != tc.known {
			t.Errorf("KnownMetaKey(%q)=%v, want %v", tc.key, got, tc.known)
		}
	}
}

func TestRecordSet(t *testing.T) {
	var rec Record
	rec.Set(MetaCountry, "US")
	rec.Set("ASN", "AS15169")
	rec.Set("unknown", "x")
	if rec.Country != "US" || rec.Asn != "AS15169" {
		t.Errorf("Set: %+v", rec)
	}
}

func TestRecordKeep(t *testing.T) {
	rec := Record{Country: "US", Region: "California", Asn: "AS15169"}
	all := rec.Keep(nil)
	if all != rec {
		t.Errorf("Keep(nil): %+v", all)
	}
	asnOnly := rec.Keep([]string{MetaAsn})
	if asnOnly.Asn != "AS15169" || asnOnly.Country != "" || asnOnly.Region != "" {
		t.Errorf("Keep(asn): %+v", asnOnly)
	}
}
