// Package backup implements SQLite online backups for Neuralwire (STY-94).
//
// It uses SQLite's VACUUM INTO to snapshot the database without locking
// readers/writers, compresses the snapshot with gzip (text-heavy data
// compresses ~90%), and prunes old backups to a retention limit.
package backup

import (
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Snapshot writes a consistent online backup of db to dstPath using
// VACUUM INTO. The database keeps serving reads/writes while the snapshot
// runs. It returns the number of bytes written.
func Snapshot(db *sql.DB, dstPath string) (int64, error) {
	if _, err := db.Exec(`VACUUM INTO ?`, dstPath); err != nil {
		return 0, fmt.Errorf("vacuum into: %w", err)
	}
	if fi, err := os.Stat(dstPath); err == nil {
		return fi.Size(), nil
	}
	return 0, nil
}

// Create compresses the database into a gzip snapshot file and returns the
// destination path. The snapshot is stored in dir with the name
// neuralwire-YYYYMMDD-HHMMSS.db.gz.
func Create(db *sql.DB, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	ts := time.Now().UTC().Format("20060102-150405")
	rawPath := filepath.Join(dir, "snapshot-"+ts+".db")
	gzPath := filepath.Join(dir, "neuralwire-"+ts+".db.gz")

	if _, err := Snapshot(db, rawPath); err != nil {
		return "", err
	}
	defer os.Remove(rawPath)

	if err := gzipFile(rawPath, gzPath); err != nil {
		return "", err
	}
	return gzPath, nil
}

// gzipFile compresses src into dst (best compression for text-heavy SQLite).
func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create gzip: %w", err)
	}
	defer out.Close()

	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("gzip writer: %w", err)
	}
	if _, err := io.Copy(gz, in); err != nil {
		gz.Close()
		return fmt.Errorf("gzip copy: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("gzip close: %w", err)
	}
	return nil
}

// Prune removes backups older than the newest retain count. Only files
// matching the neuralwire-*.db.gz pattern are considered.
func Prune(dir string, retain int) error {
	if retain <= 0 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read backup dir: %w", err)
	}
	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "neuralwire-") && strings.HasSuffix(e.Name(), ".db.gz") {
			backups = append(backups, filepath.Join(dir, e.Name()))
		}
	}
	if len(backups) <= retain {
		return nil
	}
	// Newest first (names are timestamp-sorted).
	sort.Sort(sort.Reverse(sort.StringSlice(backups)))
	for _, b := range backups[retain:] {
		_ = os.Remove(b)
	}
	return nil
}
