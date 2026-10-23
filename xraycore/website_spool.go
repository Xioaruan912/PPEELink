package xraycore

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	websiteBatchFile       = "website-report.pending.json"
	websiteBatchMaxBytes   = 4 << 20
	websiteBatchMaxBuckets = 8192
)

type websitePendingBatch struct {
	ID      string                 `json:"id"`
	Buckets []WebsiteTrafficBucket `json:"buckets"`
}

func validateWebsiteBatch(batch websitePendingBatch) error {
	if len(batch.ID) < 16 || len(batch.ID) > 64 {
		return fmt.Errorf("website pending batch: ID must contain 16..64 safe ASCII characters")
	}
	for _, c := range batch.ID {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return fmt.Errorf("website pending batch: unsafe ID")
		}
	}
	if len(batch.Buckets) == 0 || len(batch.Buckets) > websiteBatchMaxBuckets {
		return fmt.Errorf("website pending batch: bucket count must be 1..%d", websiteBatchMaxBuckets)
	}
	for i, b := range batch.Buckets {
		if b.BucketStart < 0 || b.BucketStart%3600 != 0 || b.Up < 0 || b.Down < 0 {
			return fmt.Errorf("website pending batch: invalid timestamp or counters at bucket %d", i)
		}
	}
	return nil
}

// loadWebsiteBatch never silently discards corrupt or oversized unacknowledged
// data. Only a missing file is an empty spool; callers must surface other errors.
func loadWebsiteBatch(dir string) (id string, buckets []WebsiteTrafficBucket, err error) {
	path := filepath.Join(dir, websiteBatchFile)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("website pending batch stat: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > websiteBatchMaxBytes {
		return "", nil, fmt.Errorf("website pending batch: not a regular file or exceeds 4 MiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", nil, fmt.Errorf("website pending batch open: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return "", nil, fmt.Errorf("website pending batch: file changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(file, websiteBatchMaxBytes+1))
	if err != nil {
		return "", nil, fmt.Errorf("website pending batch read: %w", err)
	}
	if len(raw) > websiteBatchMaxBytes {
		return "", nil, fmt.Errorf("website pending batch exceeds 4 MiB")
	}
	var batch websitePendingBatch
	if err := json.Unmarshal(raw, &batch); err != nil {
		return "", nil, fmt.Errorf("website pending batch JSON: %w", err)
	}
	if err := validateWebsiteBatch(batch); err != nil {
		return "", nil, err
	}
	return batch.ID, batch.Buckets, nil
}

// saveWebsiteBatch atomically persists a drained batch before it is reported.
// This protects pending batches across normal restarts/upgrades, not counters
// which have not yet been drained from the live collector when a process crashes.
func saveWebsiteBatch(dir, id string, buckets []WebsiteTrafficBucket) error {
	batch := websitePendingBatch{ID: id, Buckets: buckets}
	if err := validateWebsiteBatch(batch); err != nil {
		return err
	}
	raw, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("website pending batch encode: %w", err)
	}
	if len(raw) > websiteBatchMaxBytes {
		return fmt.Errorf("website pending batch exceeds 4 MiB")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("website pending batch directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("website pending batch: directory must not be a symlink")
	}
	// CreateTemp uses an unpredictable same-directory name and mode 0600. The
	// rename replaces a destination symlink itself instead of following it.
	file, err := os.CreateTemp(dir, ".website-report-*.tmp")
	if err != nil {
		return fmt.Errorf("website pending batch temporary file: %w", err)
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return fmt.Errorf("website pending batch permissions: %w", err)
	}
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return fmt.Errorf("website pending batch write: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("website pending batch sync: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("website pending batch close: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, websiteBatchFile)); err != nil {
		return fmt.Errorf("website pending batch rename: %w", err)
	}
	return syncWebsiteBatchDirectory(dir)
}

func clearWebsiteBatch(dir string) error {
	if err := os.Remove(filepath.Join(dir, websiteBatchFile)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("website pending batch remove: %w", err)
	}
	return syncWebsiteBatchDirectory(dir)
}

func syncWebsiteBatchDirectory(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("website pending batch open directory: %w", err)
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		return fmt.Errorf("website pending batch sync directory: %w", err)
	}
	return nil
}
