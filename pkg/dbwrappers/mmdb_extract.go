package dbwrappers

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
)

// stringLeaf is the dest type for FieldTypeString (default).
var stringLeaf = reflect.TypeOf("")

// uint32Leaf is the dest type for FieldTypeUint32 (MaxMind ASN).
var uint32Leaf = reflect.TypeOf(uint32(0))

// destType is the reflect type for this Field's MMDB scalar.
func destType(field Field) reflect.Type {
	if field.scalarType() == FieldTypeUint32 {
		return uint32Leaf
	}
	return stringLeaf
}

// mmdbExtract is a FieldMap compiled into a dest that names only those paths.
type mmdbExtract struct {
	typ   reflect.Type
	reads []mmdbRead
	pool  sync.Pool
}

// mmdbRead pulls one Record key from the generated dest.
type mmdbRead struct {
	recordKey string
	steps     []mmdbStep
}

// mmdbStep is one struct field, then an optional slice index.
type mmdbStep struct {
	field int
	index int // -1 if this step is not a slice
}

// extractNode is one MMDB map or slice while compiling a dest type.
type extractNode struct {
	kids     map[string]*extractNode
	elem     *extractNode
	leaf     bool
	leafType reflect.Type
	order    []string
}

var extractCache sync.Map

// pinnedExtract keeps fields alive so the map pointer stays a valid cache key.
type pinnedExtract struct {
	fields  FieldMap
	extract *mmdbExtract
}

var extractByPtr sync.Map

// extractCacheKey is a stable id for one FieldMap.
func extractCacheKey(fields FieldMap) string {
	parts := make([]string, 0, len(fields))
	for path, field := range fields {
		parts = append(parts, path+"="+field.Key+":"+field.scalarType())
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}

// lookupExtract returns the compiled dest for this FieldMap.
func lookupExtract(fields FieldMap) *mmdbExtract {
	ptr := reflect.ValueOf(fields).Pointer()
	if ptr != 0 {
		if pinned, ok := extractByPtr.Load(ptr); ok {
			return pinned.(*pinnedExtract).extract
		}
	}
	key := extractCacheKey(fields)
	if cached, ok := extractCache.Load(key); ok {
		stored := cached.(*mmdbExtract)
		pinExtract(ptr, fields, stored)
		return stored
	}
	built := compileExtract(fields)
	held, _ := extractCache.LoadOrStore(key, built)
	stored := held.(*mmdbExtract)
	pinExtract(ptr, fields, stored)
	return stored
}

// pinExtract remembers this map pointer so the next Lookup skips the string key.
func pinExtract(ptr uintptr, fields FieldMap, extract *mmdbExtract) {
	if ptr == 0 {
		return
	}
	extractByPtr.Store(ptr, &pinnedExtract{fields: fields, extract: extract})
}

// compileExtract builds a dest type whose maxminddb tags are the FieldMap paths.
func compileExtract(fields FieldMap) *mmdbExtract {
	root := &extractNode{kids: map[string]*extractNode{}}
	paths := make([]string, 0, len(fields))
	for path := range fields {
		paths = append(paths, path)
		addExtractPath(root, strings.Split(path, "."), fields[path])
	}
	sort.Strings(paths)
	typ := buildExtractType(root)
	reads := make([]mmdbRead, 0, len(paths))
	for _, path := range paths {
		reads = append(reads, mmdbRead{
			recordKey: fields[path].Key,
			steps:     extractSteps(root, strings.Split(path, ".")),
		})
	}
	extract := &mmdbExtract{typ: typ, reads: reads}
	extract.pool.New = func() any {
		return reflect.New(extract.typ).Interface()
	}
	return extract
}

// addExtractPath inserts one dotted path (numeric parts are slice indexes).
func addExtractPath(root *extractNode, parts []string, field Field) {
	cur := root
	for i, part := range parts {
		if _, err := strconv.Atoi(part); err == nil {
			if cur.elem == nil {
				cur.elem = &extractNode{kids: map[string]*extractNode{}}
			}
			cur = cur.elem
			continue
		}
		if cur.kids == nil {
			cur.kids = map[string]*extractNode{}
		}
		next := cur.kids[part]
		if next == nil {
			next = &extractNode{kids: map[string]*extractNode{}}
			cur.kids[part] = next
			cur.order = append(cur.order, part)
		}
		cur = next
		if i == len(parts)-1 {
			cur.leaf = true
			cur.leafType = destType(field)
		}
	}
}

// buildExtractType is the reflect type for this node (struct, slice, or any).
func buildExtractType(n *extractNode) reflect.Type {
	if n.elem != nil && len(n.kids) == 0 && !n.leaf {
		return reflect.SliceOf(buildExtractType(n.elem))
	}
	if n.leaf && len(n.kids) == 0 && n.elem == nil {
		if n.leafType != nil {
			return n.leafType
		}
		return stringLeaf
	}
	sort.Strings(n.order)
	fields := make([]reflect.StructField, 0, len(n.order))
	for i, tag := range n.order {
		fields = append(fields, reflect.StructField{
			Name: "F" + strconv.Itoa(i),
			Type: buildExtractType(n.kids[tag]),
			Tag:  reflect.StructTag(`maxminddb:"` + tag + `"`),
		})
	}
	return reflect.StructOf(fields)
}

// extractSteps walks the compiled tree to the leaf for this path.
func extractSteps(root *extractNode, parts []string) []mmdbStep {
	cur := root
	var steps []mmdbStep
	for _, part := range parts {
		if idx, err := strconv.Atoi(part); err == nil {
			if len(steps) == 0 {
				return nil
			}
			steps[len(steps)-1].index = idx
			cur = cur.elem
			continue
		}
		sort.Strings(cur.order)
		field := -1
		for i, tag := range cur.order {
			if tag == part {
				field = i
				break
			}
		}
		if field < 0 {
			return nil
		}
		steps = append(steps, mmdbStep{field: field, index: -1})
		cur = cur.kids[part]
	}
	return steps
}

// takeDest returns a zeroed dest for one Lookup.
func (e *mmdbExtract) takeDest() any {
	dest := e.pool.Get()
	reflect.ValueOf(dest).Elem().SetZero()
	return dest
}

// putDest returns dest to the pool after record() has copied leaves.
func (e *mmdbExtract) putDest(dest any) {
	e.pool.Put(dest)
}

// record reads used paths from dest after a struct Lookup.
func (e *mmdbExtract) record(dest any) dbprovider.Record {
	var rec dbprovider.Record
	root := reflect.ValueOf(dest).Elem()
	for _, read := range e.reads {
		cur := root
		ok := true
		for _, step := range read.steps {
			if !cur.IsValid() || cur.Kind() != reflect.Struct {
				ok = false
				break
			}
			cur = cur.Field(step.field)
			if step.index >= 0 {
				if cur.Kind() != reflect.Slice || step.index >= cur.Len() {
					ok = false
					break
				}
				cur = cur.Index(step.index)
			}
		}
		if !ok || !cur.IsValid() {
			continue
		}
		if cur.Kind() == reflect.String {
			if text := strings.TrimSpace(cur.String()); text != "" {
				rec.Set(read.recordKey, text)
			}
			continue
		}
		if text := stringifyMMDB(read.recordKey, cur.Interface()); text != "" {
			rec.Set(read.recordKey, text)
		}
	}
	return rec
}
