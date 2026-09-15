package dbwrappers

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

// staleVersionAge is how old an opened file's own version date may be before the wrapper warns.
const staleVersionAge = 60 * 24 * time.Hour

// readGrace is how long a handle a swap replaced may still be read by a lookup that
// already copied it. No format locks the request path, so no swap waits for those
// readers: a vendor lookup is short and 10s is more than enough.
const readGrace = 10 * time.Second

// disposeAfterGrace closes the handle a swap replaced and then releases the file that
// handle was reading, once the lookups holding it have had that read grace to finish.
func disposeAfterGrace(closeReplaced func(), retire func()) {
	go func() {
		time.Sleep(readGrace)
		closeReplaced()
		retire()
	}()
}

// lifecycleTestHoldFirstUpdate, when set by tests, runs once on the first keep-current
// delivery, before that file is published.
var lifecycleTestHoldFirstUpdate func()

// databaseFormat is what the shared lifecycle needs from one database format. The
// lifecycle decides which file is opened and when; the format decides how a handle
// becomes live, what it replaces, and how it is disposed.
type databaseFormat interface {
	// publishFile opens path as the live handle, with sourcePath as the catalog or seed
	// file it came from, and disposes the handle it replaced. retire is called once that
	// replaced handle is closed and the file it was reading can be removed. version is
	// nil for a format with no version header. published is false when this wrapper is
	// already closed: nothing became live and retire is not called.
	publishFile(path, sourcePath string, retire func()) (version *dbutils.DBVersion, published bool, err error)
	// refusePublish marks this wrapper closed so a publish that lands later cannot go live.
	refusePublish()
	// closeHandle closes the live handle. The keep-current loop has already stopped.
	closeHandle()
}

// openTarget is one file to open and how: the file handed to the SDK, the catalog or
// seed file it came from, the temp copy to dispose (empty when opened in place), and
// why that handle becomes live.
type openTarget struct {
	openPath   string
	sourcePath string
	localCopy  string
	reason     openReason
}

// lifecycle is the format-agnostic half of a wrapper: resolve the file for one catalog
// source, open the first handle, keep it current, and log one line per published handle.
type lifecycle struct {
	format databaseFormat
	// subject names this wrapper on its log lines: BIN or MMDB.
	subject string
	// servesFromFile is true when a published handle keeps reading the file it opened, so
	// a dated catalog file is served from a temp copy the keep-current writer cannot replace.
	servesFromFile bool
	// seedFirst opens the bundled defaultFile while a dated file is still pending, so New
	// does not wait for the copy of that dated file.
	seedFirst bool
	// allowMissing lets this wrapper start with no file at all and wait for auto-update.
	allowMissing bool
	source       dbsource.Config
	logger       *slog.Logger
	updater      *dbsource.Updater
	// publishedSource is the catalog or seed file the live handle came from. The
	// keep-current loop compares against it. May be stale during a swap. Accepted.
	publishedSource string
	// publishedCopy is the temp copy the live handle reads, empty when opened in place.
	publishedCopy string
}

// initialize opens the first handle for this source: the bundled seed while a dated file
// is still pending, a temp copy of that dated file, or the resolved file in place.
func (l *lifecycle) initialize() error {
	resolved, err := dbsource.Resolve(l.source, l.logger)
	if err != nil && resolved == "" && !l.allowMissing {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}
	if resolved == "" {
		if !l.allowMissing {
			return fmt.Errorf("database file not found")
		}
		l.logger.Info("no database file yet; waiting for auto-update")
		return nil
	}
	version, err := l.publish(l.chooseStartupTarget(resolved))
	if err != nil {
		return err
	}
	l.warnStaleVersion(version)
	return nil
}

// chooseStartupTarget picks the file initialize opens for an already resolved path. Only
// a dated catalog file is copied, and only for a format that keeps reading what it opened.
func (l *lifecycle) chooseStartupTarget(resolved string) openTarget {
	inPlace := openTarget{openPath: resolved, sourcePath: resolved, reason: reasonInit}
	latest, err := dbsource.Latest(l.source.Dir, l.source.Key, l.source.DatabaseType)
	if err != nil || latest == "" || latest != resolved {
		// Operator seed path or bundled default: no writer replaces it, open it in place.
		return inPlace
	}
	if l.seedFirst {
		if seed, serr := dbsource.BundledFile(l.source, l.logger); serr == nil && seed != "" {
			// Serve the seed so New returns; the keep-current loop promotes the dated file.
			return openTarget{openPath: seed, sourcePath: seed, reason: reasonSeed}
		}
	}
	if !l.servesFromFile {
		return inPlace
	}
	copied, cerr := l.createLocalCopy(latest)
	if cerr != nil {
		l.logger.Warn("local copy failed, opening source", "error", cerr)
		return inPlace
	}
	return openTarget{openPath: copied, sourcePath: latest, localCopy: copied, reason: reasonInit}
}

// publish opens target as the live handle and logs the one line for it. The temp copy
// this publish replaces is removed once the format has closed the handle reading it.
func (l *lifecycle) publish(target openTarget) (*dbutils.DBVersion, error) {
	replacedCopy := l.publishedCopy
	version, published, err := l.format.publishFile(target.openPath, target.sourcePath, func() {
		l.discardCopy(replacedCopy)
	})
	if err != nil {
		l.discardCopy(target.localCopy)
		return nil, err
	}
	if !published {
		l.discardCopy(target.localCopy)
		return version, nil
	}
	l.publishedSource = target.sourcePath
	l.publishedCopy = target.localCopy
	l.logOpened(target, version)
	return version, nil
}

// logOpened is the single line per published handle: why it is live, the file the SDK
// opened, where that file came from, its version when the format has one, and its size.
func (l *lifecycle) logOpened(target openTarget, version *dbutils.DBVersion) {
	attrs := []any{
		"reason", string(target.reason),
		"path", target.openPath,
		"source_path", target.sourcePath,
	}
	if version != nil {
		attrs = append(attrs, "version", version.String())
	}
	attrs = append(attrs, "size_bytes", openedFileSize(target.openPath))
	l.logger.Info(l.subject+" opened", attrs...)
}

// warnStaleVersion warns when the opened file's own version date is over two months old.
// A format with no version header has nothing to warn about.
func (l *lifecycle) warnStaleVersion(version *dbutils.DBVersion) {
	if version == nil {
		return
	}
	age := time.Since(version.Date())
	if age <= staleVersionAge {
		return
	}
	l.logger.Warn(l.subject+" database is more than 2 months old",
		"version", version.String(),
		"age", age.Round(24*time.Hour))
}

// startUpdate starts the keep-current loop: promote a dated file already on disk, then
// GET when that file is older than MinAge. Each new file is published as a hot swap.
func (l *lifecycle) startUpdate() {
	updater, err := dbsource.Start(l.source, l.logger, func(path string, trigger dbsource.UpdateTrigger) {
		// Tests hold the first delivery so New can return while the seed is still live.
		if hold := lifecycleTestHoldFirstUpdate; hold != nil {
			lifecycleTestHoldFirstUpdate = nil
			hold()
		}
		if path == "" || path == l.publishedSource {
			return
		}
		if err := l.hotSwap(path, trigger); err != nil {
			l.logger.Error("failed to perform hot swap", "error", err)
		}
	})
	if err != nil {
		l.logger.Error("source updater", "error", err)
		return
	}
	l.updater = updater
}

// hotSwap replaces the live handle with newDatabasePath, through a temp copy when the
// format keeps reading the file it opened. trigger names the swap on the log line.
func (l *lifecycle) hotSwap(newDatabasePath string, trigger dbsource.UpdateTrigger) error {
	target := openTarget{
		openPath:   newDatabasePath,
		sourcePath: newDatabasePath,
		reason:     updaterReason(trigger),
	}
	if l.servesFromFile {
		copied, err := l.createLocalCopy(newDatabasePath)
		if err != nil {
			return err
		}
		target.openPath = copied
		target.localCopy = copied
	}
	_, err := l.publish(target)
	return err
}

// sleep stops the keep-current loop while this wrapper is parked in reclaim grace.
func (l *lifecycle) sleep() {
	l.updater.Stop()
}

// wake starts one keep-current loop again after a reclaim, once the previous Stop returned.
func (l *lifecycle) wake() {
	l.startUpdate()
}

// close stops the keep-current loop, closes the live handle, and removes the temp copy
// that handle was reading. Refuse first so an update landing during Stop cannot publish.
func (l *lifecycle) close() {
	l.format.refusePublish()
	l.updater.Stop()
	l.format.closeHandle()
	l.discardCopy(l.publishedCopy)
	l.publishedCopy = ""
}

// sourcePath is the catalog or seed file the live handle came from.
func (l *lifecycle) sourcePath() string {
	return l.publishedSource
}

// createLocalCopy writes a process-temp copy of sourcePath for this source.
func (l *lifecycle) createLocalCopy(sourcePath string) (string, error) {
	name := copyName(l.source.DatabaseType, fileToken(l.source.Key), time.Now().UnixNano())
	tmpFile := filepath.Join(os.TempDir(), name)
	if err := fileutils.Copy(sourcePath, tmpFile, false); err != nil {
		return "", fmt.Errorf("failed to create local copy: %w", err)
	}
	return tmpFile, nil
}

// discardCopy removes a temp copy no handle reads any more. Empty means opened in place.
func (l *lifecycle) discardCopy(localCopy string) {
	if localCopy == "" {
		return
	}
	if err := os.Remove(localCopy); err != nil && !os.IsNotExist(err) {
		l.logger.Debug("failed to remove temp database copy", "path", localCopy, "error", err)
	}
}

// copyName is the temp-copy basename: format, catalog key, Unix nanosecond, format suffix.
func copyName(databaseType, token string, unixNano int64) string {
	if token == "" {
		return fmt.Sprintf("%s_%d%s", databaseType, unixNano, dbsource.Ext(databaseType))
	}
	return fmt.Sprintf("%s_%s_%d%s", databaseType, token, unixNano, dbsource.Ext(databaseType))
}

// fileToken is the catalog key as a temp-file name segment.
func fileToken(catalogKey string) string {
	var b strings.Builder
	for _, r := range catalogKey {
		if fileTokenRune(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}

// fileTokenRune reports whether r may appear in a temp-copy catalog-key segment.
func fileTokenRune(r rune) bool {
	return r == '.' || r == '_' || r == '-' ||
		(r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// openedFileSize is the byte length of path, or 0 when Stat fails.
func openedFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
