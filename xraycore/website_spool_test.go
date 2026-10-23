package xraycore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func spoolFixture() (string, []WebsiteTrafficBucket) {
	return "batch_0123456789-abcdef", []WebsiteTrafficBucket{
		{BucketStart: 1767225600, InboundTag: "in-17", Domain: "example.com", Up: 9007199254740993, Down: 27},
		{BucketStart: 1767229200, InboundTag: "in-18", Domain: "unknown", Up: 0, Down: 500},
	}
}

func TestWebsiteSpoolRoundTripAndClear(t *testing.T) {
	dir := t.TempDir()
	if id, buckets, err := loadWebsiteBatch(dir); err != nil || id != "" || buckets != nil {
		t.Fatalf("missing spool: %q %+v %v", id, buckets, err)
	}
	id, buckets := spoolFixture()
	if err := saveWebsiteBatch(dir, id, buckets); err != nil {
		t.Fatal(err)
	}
	// Read afresh with no collector/process state: even int64s above JS's exact
	// integer range and the ordering/identity of the batch must survive intact.
	gotID, got, err := loadWebsiteBatch(dir)
	if err != nil || gotID != id || !reflect.DeepEqual(got, buckets) {
		t.Fatalf("restore: %q %+v %v", gotID, got, err)
	}
	info, err := os.Stat(filepath.Join(dir, websiteBatchFile))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("unsafe permissions: %o", info.Mode().Perm())
	}
	buckets[0].Up++
	if err := saveWebsiteBatch(dir, id+"2", buckets); err != nil {
		t.Fatal(err)
	}
	gotID, got, err = loadWebsiteBatch(dir)
	if err != nil || gotID != id+"2" || !reflect.DeepEqual(got, buckets) {
		t.Fatalf("atomic replacement: %q %+v %v", gotID, got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v %v", entries, err)
	}
	for i := 0; i < 2; i++ {
		if err := clearWebsiteBatch(dir); err != nil {
			t.Fatal(err)
		}
	}
	if id, buckets, err := loadWebsiteBatch(dir); err != nil || id != "" || buckets != nil {
		t.Fatalf("cleared spool: %q %+v %v", id, buckets, err)
	}
}

func TestWebsiteSpoolRejectsCorruptAndOversized(t *testing.T) {
	id, buckets := spoolFixture()
	valid, _ := json.Marshal(websitePendingBatch{ID: id, Buckets: buckets})
	for name, raw := range map[string][]byte{
		"invalid JSON":  []byte("{"),
		"null":          []byte("null"),
		"trailing data": append(append([]byte(nil), valid...), []byte(" {}")...),
		"oversized":     []byte(strings.Repeat(" ", websiteBatchMaxBytes+1)),
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, websiteBatchFile)
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := loadWebsiteBatch(dir); err == nil {
				t.Fatal("invalid pending data silently accepted")
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("invalid pending data was deleted: %v", err)
			}
		})
	}
}

func TestWebsiteSpoolValidatesBeforeSaveAndAfterLoad(t *testing.T) {
	id, buckets := spoolFixture()
	cases := map[string]websitePendingBatch{
		"short ID":            {ID: "short", Buckets: buckets},
		"long ID":             {ID: strings.Repeat("a", 65), Buckets: buckets},
		"unsafe ID":           {ID: strings.Repeat("a", 16) + "/", Buckets: buckets},
		"unicode ID":          {ID: strings.Repeat("a", 16) + "é", Buckets: buckets},
		"empty":               {ID: id},
		"too many":            {ID: id, Buckets: make([]WebsiteTrafficBucket, websiteBatchMaxBuckets+1)},
		"negative timestamp":  {ID: id, Buckets: []WebsiteTrafficBucket{{BucketStart: -3600}}},
		"unaligned timestamp": {ID: id, Buckets: []WebsiteTrafficBucket{{BucketStart: 1767225601}}},
		"negative up":         {ID: id, Buckets: []WebsiteTrafficBucket{{Up: -1}}},
		"negative down":       {ID: id, Buckets: []WebsiteTrafficBucket{{Down: -1}}},
		"encoded too large":   {ID: id, Buckets: []WebsiteTrafficBucket{{Domain: strings.Repeat("a", websiteBatchMaxBytes)}}},
	}
	for name, batch := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := saveWebsiteBatch(dir, batch.ID, batch.Buckets); err == nil {
				t.Fatal("invalid save accepted")
			}
			raw, err := json.Marshal(batch)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, websiteBatchFile), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := loadWebsiteBatch(dir); err == nil {
				t.Fatal("invalid restored batch accepted")
			}
		})
	}
}

func TestWebsiteSpoolRejectsDirectorySymlink(t *testing.T) {
	realDir := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	id, buckets := spoolFixture()
	if err := saveWebsiteBatch(alias, id, buckets); err == nil {
		t.Fatal("symlink directory accepted")
	}
	if _, err := os.Stat(filepath.Join(realDir, websiteBatchFile)); !os.IsNotExist(err) {
		t.Fatalf("write escaped directory: %v", err)
	}
}

func TestWebsiteSpoolRejectedSavePreservesPending(t *testing.T) {
	dir := t.TempDir()
	id, buckets := spoolFixture()
	if err := saveWebsiteBatch(dir, id, buckets); err != nil {
		t.Fatal(err)
	}
	if err := saveWebsiteBatch(dir, "invalid", buckets); err == nil {
		t.Fatal("invalid replacement accepted")
	}
	gotID, got, err := loadWebsiteBatch(dir)
	if err != nil || gotID != id || !reflect.DeepEqual(got, buckets) {
		t.Fatalf("pending data lost on failed replacement: %q %+v %v", gotID, got, err)
	}
}

func TestWebsiteSpoolDoesNotFollowDestinationSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "untouched")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, websiteBatchFile)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, _, err := loadWebsiteBatch(dir); err == nil {
		t.Fatal("symlink read accepted")
	}
	id, buckets := spoolFixture()
	if err := saveWebsiteBatch(dir, id, buckets); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(outside)
	if err != nil || string(raw) != "secret" {
		t.Fatalf("write escaped spool directory: %q %v", raw, err)
	}
	if _, _, err := loadWebsiteBatch(dir); err != nil {
		t.Fatalf("replacement is not a valid regular spool: %v", err)
	}
}
