package dbwrappers

import (
	"reflect"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
)

func TestCompileExtract_NestedAndIndex(t *testing.T) {
	fields := FieldMap{
		"country.iso_code":        {Key: dbprovider.MetaCountry},
		"country.names.en":        {Key: dbprovider.MetaCountryName},
		"subdivisions.0.iso_code": {Key: dbprovider.MetaRegion},
	}
	extract := compileExtract(fields)
	if extract.typ.Kind() != reflect.Struct {
		t.Fatalf("root kind: %s", extract.typ.Kind())
	}

	var region mmdbRead
	for _, read := range extract.reads {
		if read.recordKey == dbprovider.MetaRegion {
			region = read
		}
	}
	if len(region.steps) != 2 {
		t.Fatalf("subdivisions steps: %+v", region.steps)
	}
	if region.steps[0].index != 0 {
		t.Fatalf("slice index: %+v", region.steps[0])
	}
	if region.steps[1].index != -1 {
		t.Fatalf("iso_code should not be a slice: %+v", region.steps[1])
	}
}

func TestLookupExtract_CachesSameMap(t *testing.T) {
	fields := FieldMap{"country_code": {Key: dbprovider.MetaCountry}}
	first := lookupExtract(fields)
	second := lookupExtract(FieldMap{"country_code": {Key: dbprovider.MetaCountry}})
	if first != second {
		t.Fatal("same FieldMap must reuse one compiled dest")
	}
}

func TestCompileExtract_Uint32Leaf(t *testing.T) {
	extract := compileExtract(FieldMap{
		"autonomous_system_number": {Key: dbprovider.MetaAsn, Type: FieldTypeUint32},
	})
	if extract.typ.NumField() != 1 || extract.typ.Field(0).Type != uint32Leaf {
		t.Fatalf("uint32 leaf: %s", extract.typ)
	}
}
