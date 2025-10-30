package geodb

// Config contains the configuration needed for database factory management
type Config struct {
	DatabaseFilePath        string
	DatabaseAutoUpdate      bool
	DatabaseAutoUpdateDir   string
	DatabaseAutoUpdateToken string
	DatabaseAutoUpdateCode  string
}
