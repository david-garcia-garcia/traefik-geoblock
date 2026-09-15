# Standards

1. [hard] Leave a trail — `pkg/geoblock/ip_blocks.go:45` — `insertBlocksFromDirectory` is a new method with no succinct job comment (`func insertBlocksFromDirectory(helper *iplookup.Helper, directoryPath string, logger *slog.Logger) (int, error)`)
   → Add a one-line comment that it walks `.txt` files under the directory and stores each CIDR on the helper
   Status: done
   Argument: added job comment on insertBlocksFromDirectory.
2. [hard] Leave a trail — `pkg/geoblock/ip_blocks.go:87` — `readBlocksFromFile` is a new method with no succinct job comment (`func readBlocksFromFile(filePath string, logger *slog.Logger) ([]string, error)`)
   → Add a one-line comment that it returns CIDR lines from the file, skipping blanks, `#` comments, and invalid CIDRs
   Status: done
   Argument: added job comment on readBlocksFromFile.
