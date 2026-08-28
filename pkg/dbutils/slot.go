package dbutils

const (
	SlotGeo = "geo"
	SlotASN = "asn"
)

// DatedSlotGlob is a filename glob for YYYYMMDD_<slot><ext>.
func DatedSlotGlob(slot, ext string) string {
	return "*_" + slot + ext
}
