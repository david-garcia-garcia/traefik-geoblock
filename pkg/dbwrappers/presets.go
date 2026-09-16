package dbwrappers

// registerPresets adds named vendor maps for every binary type.
func registerPresets() {
	registerBINPresets()
	registerMMDBPresets()
}
