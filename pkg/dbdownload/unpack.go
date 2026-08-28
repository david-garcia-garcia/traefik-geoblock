package dbdownload

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/oschwald/maxminddb-golang"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
)

func unpackBody(r io.Reader, destDir, archive, databaseType string) (string, error) {
	switch archive {
	case ArchiveNone:
		out := filepath.Join(destDir, "payload"+Ext(databaseType))
		f, err := os.Create(out)
		if err != nil {
			return "", fmt.Errorf("failed to create download file: %w", err)
		}
		if _, err := io.Copy(f, dbutils.LimitBody(r)); err != nil {
			f.Close()
			return "", fmt.Errorf("failed to save download file: %w", err)
		}
		if err := f.Close(); err != nil {
			return "", err
		}
		return out, nil
	case ArchiveZIP:
		return unpackZIP(r, destDir, databaseType)
	case ArchiveTarGz:
		return unpackTarGz(r, destDir, databaseType)
	default:
		return "", fmt.Errorf("unknown archive %q", archive)
	}
}

func unpackZIP(r io.Reader, destDir, databaseType string) (string, error) {
	zipPath := filepath.Join(destDir, "archive.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to create zip file: %w", err)
	}
	if _, err := io.Copy(zf, dbutils.LimitBody(r)); err != nil {
		zf.Close()
		return "", fmt.Errorf("failed to save zip file: %w", err)
	}
	if err := zf.Close(); err != nil {
		return "", err
	}
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()
	want := strings.ToLower(Ext(databaseType))
	for _, file := range reader.File {
		if strings.ToLower(filepath.Ext(file.Name)) != want {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open zip entry: %w", err)
		}
		out := filepath.Join(destDir, "payload"+Ext(databaseType))
		w, err := os.Create(out)
		if err != nil {
			rc.Close()
			return "", fmt.Errorf("failed to create extracted file: %w", err)
		}
		_, err = io.Copy(w, io.LimitReader(rc, dbutils.HTTPGetMaxBytes)) // #nosec G110
		rc.Close()
		if err != nil {
			w.Close()
			return "", fmt.Errorf("failed to extract archive: %w", err)
		}
		if err := w.Close(); err != nil {
			return "", err
		}
		return out, nil
	}
	return "", fmt.Errorf("archive contained no %s", Ext(databaseType))
}

func unpackTarGz(r io.Reader, destDir, databaseType string) (string, error) {
	gz, err := gzip.NewReader(dbutils.LimitBody(r))
	if err != nil {
		return "", fmt.Errorf("download is not gzip: %w", err)
	}
	defer gz.Close()
	want := strings.ToLower(Ext(databaseType))
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Base(filepath.ToSlash(hdr.Name))
		if strings.Contains(hdr.Name, "..") || strings.ToLower(filepath.Ext(name)) != want {
			continue
		}
		out := filepath.Join(destDir, "payload"+Ext(databaseType))
		f, err := os.Create(out)
		if err != nil {
			return "", fmt.Errorf("failed to create extracted file: %w", err)
		}
		if _, err := io.Copy(f, io.LimitReader(tr, dbutils.HTTPGetMaxBytes)); err != nil {
			f.Close()
			return "", fmt.Errorf("failed to extract archive: %w", err)
		}
		if err := f.Close(); err != nil {
			return "", err
		}
		return out, nil
	}
	return "", fmt.Errorf("archive contained no %s", Ext(databaseType))
}

func fileDate(path, databaseType string) (string, error) {
	switch databaseType {
	case TypeBIN:
		version, err := dbutils.GetDatabaseVersion(path)
		if err != nil {
			return "", fmt.Errorf("invalid BIN file: %w", err)
		}
		return version.Date().Format("20060102"), nil
	case TypeMMDB:
		buf, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read MMDB: %w", err)
		}
		reader, err := maxminddb.FromBytes(buf)
		if err != nil {
			return "", fmt.Errorf("invalid MMDB file: %w", err)
		}
		buildDate, err := dbutils.MMDBBuildDate(reader.Metadata.BuildEpoch)
		_ = reader.Close()
		if err != nil {
			return "", err
		}
		return buildDate.Format("20060102"), nil
	default:
		return "", fmt.Errorf("unknown databaseType %q", databaseType)
	}
}
