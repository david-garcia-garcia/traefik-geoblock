package dbwrappers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
)

// FieldMap is on-disk path → Record key (country, asn, …).
type FieldMap map[string]string

// Clone returns a copy.
func (m FieldMap) Clone() FieldMap {
	if m == nil {
		return nil
	}
	out := make(FieldMap, len(m))
	for path, key := range m {
		out[path] = key
	}
	return out
}

// hasPath reports whether the map names that on-disk path.
func (m FieldMap) hasPath(path string) bool {
	_, ok := m[path]
	return ok
}

// apply puts source values onto rec using the map. Empty source values are skipped.
func (m FieldMap) apply(rec *dbprovider.Record, value func(path string) string) {
	for path, key := range m {
		if got := value(path); got != "" {
			rec.Set(key, got)
		}
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

// walkPath follows dotted keys and numeric indexes on a decoded MMDB map.
func walkPath(root any, path string) any {
	cur := root
	for _, part := range strings.Split(path, ".") {
		if cur == nil {
			return nil
		}
		if idx, err := strconv.Atoi(part); err == nil {
			list, ok := cur.([]any)
			if !ok || idx < 0 || idx >= len(list) {
				return nil
			}
			cur = list[idx]
			continue
		}
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = obj[part]
	}
	return cur
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
