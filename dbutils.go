package traefik_geoblock

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type DBVersion struct {
	Type         byte
	ColumnWidth4 byte
	Year         byte
	Month        byte
	Day          byte
	ProductCode  byte
	LicenseCode  byte
	DatabaseSize byte
	IPCount4     uint32
	IPBase4      uint32
	IPCount6     uint32
	IPBase6      uint32
	IndexBase4   uint32
	IndexBase6   uint32
}

func (v DBVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Year, v.Month, v.Day)
}

// Date returns a time.Time representing the database version date
func (v DBVersion) Date() time.Time {
	// The year is stored as an offset from 2000
	year := 2000 + int(v.Year)
	return time.Date(year, time.Month(v.Month), int(v.Day), 0, 0, 0, 0, time.UTC)
}

// GetDatabaseVersion reads the version information from an IP2Location database file.
// It returns the version information or an error if the version cannot be read.
func GetDatabaseVersion(filepath string) (*DBVersion, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database file: %w", err)
	}
	defer file.Close()

	// Read first 512 bytes of header
	headerBytes := make([]byte, 512)
	n, err := file.Read(headerBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read header bytes: %w", err)
	}
	if n != 512 {
		return nil, fmt.Errorf("incomplete header read: got %d bytes, expected 512", n)
	}

	version := &DBVersion{
		Type:         headerBytes[0] - 1,
		ColumnWidth4: headerBytes[1] * 4,
		Year:         headerBytes[2],
		Month:        headerBytes[3],
		Day:          headerBytes[4],
		ProductCode:  headerBytes[29],
		LicenseCode:  headerBytes[30],
		DatabaseSize: headerBytes[31],
		IPCount4:     binary.LittleEndian.Uint32(headerBytes[5:9]),
		IPBase4:      binary.LittleEndian.Uint32(headerBytes[9:13]),
		IPCount6:     binary.LittleEndian.Uint32(headerBytes[13:17]),
		IPBase6:      binary.LittleEndian.Uint32(headerBytes[17:21]),
		IndexBase4:   binary.LittleEndian.Uint32(headerBytes[21:25]),
		IndexBase6:   binary.LittleEndian.Uint32(headerBytes[25:29]),
	}

	if version.ProductCode == 0 {
		return nil, fmt.Errorf("invalid IP2Location BIN file format")
	}

	return version, nil
}

// GetDateFromName extracts the date from a database filename.
// Returns the parsed time and an error if the filename doesn't match the expected format.
func GetDateFromName(dbPath string) (time.Time, error) {
	// Normalize to Linux-style paths. I know this is NOT the best way, but works for our use case.
	dbPath = strings.ReplaceAll(dbPath, "\\", "/")

	_, tail := filepath.Split(dbPath)
	parts := strings.Split(tail, "_")
	if len(parts) < 1 {
		return time.Time{}, fmt.Errorf("invalid filename format: %s", tail)
	}

	dateStr := parts[0]
	if len(dateStr) != 8 {
		return time.Time{}, fmt.Errorf("invalid date format in filename: %s", dateStr)
	}

	year, err := strconv.Atoi(dateStr[:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid year in filename: %s", dateStr[:4])
	}

	month, err := strconv.Atoi(dateStr[4:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month in filename: %s", dateStr[4:6])
	}

	day, err := strconv.Atoi(dateStr[6:8])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day in filename: %s", dateStr[6:8])
	}

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}
