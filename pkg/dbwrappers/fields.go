package dbwrappers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
)

// MMDB scalar types on a Field. Empty Type is FieldTypeString.
const (
	FieldTypeString = "string"
	FieldTypeUint32 = "uint32"
)

const (
	fieldYAMLKey  = "key"
	fieldYAMLType = "type"
)

// Field is one on-disk column: Record key and MMDB scalar type (default string).
type Field struct {
	Key  string
	Type string
}

// FieldMap is on-disk path → Field.
type FieldMap map[string]Field

// Clone returns a copy.
func (m FieldMap) Clone() FieldMap {
	if m == nil {
		return nil
	}
	out := make(FieldMap, len(m))
	for path, field := range m {
		out[path] = field
	}
	return out
}

// apply puts source values onto rec using the map. Empty source values are skipped.
func (m FieldMap) apply(rec *dbprovider.Record, value func(path string) string) {
	for path, field := range m {
		if got := value(path); got != "" {
			rec.Set(field.Key, got)
		}
	}
}

// CatalogYAML is the Traefik map: a Record key string, or {key, type} when not string.
func (m FieldMap) CatalogYAML() map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for path, field := range m {
		if field.scalarType() == FieldTypeString {
			out[path] = field.Key
			continue
		}
		out[path] = map[string]any{fieldYAMLKey: field.Key, fieldYAMLType: field.scalarType()}
	}
	return out
}

// scalarType is string when Type is empty.
func (f Field) scalarType() string {
	if strings.TrimSpace(f.Type) == "" {
		return FieldTypeString
	}
	return strings.ToLower(strings.TrimSpace(f.Type))
}

// ParseFields turns catalog YAML values into a FieldMap.
func ParseFields(raw map[string]any) (FieldMap, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(FieldMap, len(raw))
	for path, value := range raw {
		field, err := ParseField(value)
		if err != nil {
			return nil, fmt.Errorf("path %q: %w", path, err)
		}
		out[path] = field
	}
	return out, nil
}

// ParseField accepts a Record key string or a {key, type} map.
func ParseField(raw any) (Field, error) {
	switch typed := raw.(type) {
	case string:
		return Field{Key: typed, Type: FieldTypeString}, nil
	case Field:
		kind, err := normalizeFieldType(typed.Type)
		if err != nil {
			return Field{}, err
		}
		return Field{Key: typed.Key, Type: kind}, nil
	case map[string]any:
		return fieldFromObject(typed)
	case map[string]string:
		obj := make(map[string]any, len(typed))
		for key, value := range typed {
			obj[key] = value
		}
		return fieldFromObject(obj)
	default:
		return Field{}, fmt.Errorf("want a Record key or {%s, %s}, got %T", fieldYAMLKey, fieldYAMLType, raw)
	}
}

// fieldFromObject reads {key, type} from a catalog map value.
func fieldFromObject(obj map[string]any) (Field, error) {
	key, ok := obj[fieldYAMLKey].(string)
	if !ok || strings.TrimSpace(key) == "" {
		return Field{}, fmt.Errorf("%s must be a Record key string", fieldYAMLKey)
	}
	typeRaw := ""
	if v, exists := obj[fieldYAMLType]; exists && v != nil {
		text, ok := v.(string)
		if !ok {
			return Field{}, fmt.Errorf("%s must be %s or %s", fieldYAMLType, FieldTypeString, FieldTypeUint32)
		}
		typeRaw = text
	}
	kind, err := normalizeFieldType(typeRaw)
	if err != nil {
		return Field{}, err
	}
	return Field{Key: key, Type: kind}, nil
}

// normalizeFieldType is string or uint32. Empty is string.
func normalizeFieldType(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", FieldTypeString:
		return FieldTypeString, nil
	case FieldTypeUint32:
		return FieldTypeUint32, nil
	default:
		return "", fmt.Errorf("unknown field type %q (supported: %s, %s)", raw, FieldTypeString, FieldTypeUint32)
	}
}

// fieldPreset is a named vendor column map.
type fieldPreset struct {
	format string
	fields FieldMap
}

var (
	fieldPresets = map[string]fieldPreset{}
	presetsOnce  sync.Once
)

// ensurePresets registers named vendor maps on first use.
func ensurePresets() {
	presetsOnce.Do(registerPresets)
}

// Preset returns the named vendor map. Unknown names return false.
func Preset(name string) (format string, fields FieldMap, ok bool) {
	ensurePresets()
	p, ok := fieldPresets[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return "", nil, false
	}
	return p.format, p.fields.Clone(), true
}

// PresetNames is the sorted list of fieldsPreconfigured values.
func PresetNames() []string {
	ensurePresets()
	out := make([]string, 0, len(fieldPresets))
	for name := range fieldPresets {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func register(name, format string, fields FieldMap) {
	fieldPresets[name] = fieldPreset{format: format, fields: fields}
}

func registerAlias(alias, name string) {
	fieldPresets[alias] = fieldPresets[name]
}

// stringifyMMDB turns a decoded MMDB scalar into a Record string.
func stringifyMMDB(key string, val any) string {
	if val == nil {
		return ""
	}
	var s string
	switch typed := val.(type) {
	case string:
		s = typed
	case uint32:
		s = strconv.FormatUint(uint64(typed), 10)
	case uint64:
		s = strconv.FormatUint(typed, 10)
	case int32:
		s = strconv.FormatInt(int64(typed), 10)
	case int64:
		s = strconv.FormatInt(typed, 10)
	case int:
		s = strconv.Itoa(typed)
	case float64:
		s = strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if !typed {
			return ""
		}
		s = "true"
	default:
		s = fmt.Sprint(typed)
	}
	s = strings.TrimSpace(s)
	if key == dbprovider.MetaAsn && s != "" && !strings.HasPrefix(strings.ToUpper(s), "AS") {
		if _, err := strconv.ParseUint(s, 10, 64); err == nil {
			return "AS" + s
		}
	}
	return s
}
