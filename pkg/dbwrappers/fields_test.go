package dbwrappers

import (
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
)

// mustFields is the named vendor map or a fatal.
func mustFields(t *testing.T, name string) FieldMap {
	t.Helper()
	_, fields, ok := Preset(name)
	if !ok {
		t.Fatalf("unknown preset %q", name)
	}
	return fields
}

func TestPreset_KnownAndUnknown(t *testing.T) {
	format, fields, ok := Preset(PresetIPinfoLite)
	if !ok || format != dbsource.TypeMMDB || fields["country_code"] != dbprovider.MetaCountry {
		t.Fatalf("ipinfo_lite: format=%q fields=%v ok=%v", format, fields, ok)
	}
	fields["country_code"] = "mutated"
	_, again, _ := Preset(PresetIPinfoLite)
	if again["country_code"] != dbprovider.MetaCountry {
		t.Error("Preset must clone")
	}
	if _, _, ok := Preset("not-a-preset"); ok {
		t.Error("unknown preset must fail")
	}
}

func TestPreset_FormatFamilies(t *testing.T) {
	cases := []struct {
		name, format string
	}{
		{PresetIP2LocationLite, dbsource.TypeBIN},
		{PresetIP2LocationASN, dbsource.TypeBIN},
		{"ip2location_db8", dbsource.TypeBIN},
		{"ip2location_lite_db1", dbsource.TypeBIN},
		{PresetIPinfoCore, dbsource.TypeMMDB},
		{PresetMaxMindCountry, dbsource.TypeMMDB},
		{PresetMaxMindASN, dbsource.TypeMMDB},
		{"geolite2_country", dbsource.TypeMMDB},
		{"geoip2_city", dbsource.TypeMMDB},
	}
	for _, tc := range cases {
		format, fields, ok := Preset(tc.name)
		if !ok {
			t.Errorf("%s: missing", tc.name)
			continue
		}
		if format != tc.format {
			t.Errorf("%s format: got %q want %q", tc.name, format, tc.format)
		}
		if len(fields) == 0 {
			t.Errorf("%s: empty map", tc.name)
		}
	}
}

func TestPresetNames_IncludesOfficialDB8(t *testing.T) {
	names := strings.Join(PresetNames(), ",")
	for _, want := range []string{"ip2location_db8", "ipinfo_lite", "maxmind_asn"} {
		if !strings.Contains(names, want) {
			t.Errorf("PresetNames missing %s: %s", want, names)
		}
	}
}

func TestWalkPathAndStringifyMMDB(t *testing.T) {
	root := map[string]any{
		"country": map[string]any{"iso_code": "GB"},
		"list":    []any{map[string]any{"iso_code": "CA"}},
		"asn":     uint32(15169),
	}
	if got := walkPath(root, "country.iso_code"); got != "GB" {
		t.Errorf("nested: %v", got)
	}
	if got := walkPath(root, "list.0.iso_code"); got != "CA" {
		t.Errorf("index: %v", got)
	}
	if got := stringifyMMDB(dbprovider.MetaAsn, uint32(15169)); got != "AS15169" {
		t.Errorf("asn prefix: %q", got)
	}
}
