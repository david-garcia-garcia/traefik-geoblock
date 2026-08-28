package dbwrappers

// Reset closes every singleton wrapper. Tests only.
func Reset() {
	binLock.Reset(func() {
		for key, w := range binByKey {
			w.close()
			delete(binByKey, key)
		}
	})
	mmdbLock.Reset(func() {
		for key, w := range mmdbByKey {
			w.close()
			delete(mmdbByKey, key)
		}
	})
}
