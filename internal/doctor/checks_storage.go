package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ananymishradev/foam-clutch/internal/store"
	"golang.org/x/sys/unix"
)

const (
	warnFreeBytes = 10 * 1024 * 1024 * 1024
	failFreeBytes = 1 * 1024 * 1024 * 1024
)

type storagePaths struct {
	db, cache, run string
}

type storagePathsCheck struct{ paths storagePaths }

func (storagePathsCheck) Name() string { return "Storage paths" }
func (c storagePathsCheck) Run(ctx context.Context) Result {
	for _, path := range []string{c.paths.db, c.paths.cache, c.paths.run} {
		if path == "" {
			continue
		}
		if err := os.MkdirAll(path, 0o750); err != nil {
			return Result{Category: "Storage", Name: c.Name(), Status: FAIL, Detail: fmt.Sprintf("%s: %v", path, err), Hint: "choose writable DB, cache, and run directories"}
		}
		f, err := os.CreateTemp(path, ".clutch-doctor-*")
		if err != nil {
			return Result{Category: "Storage", Name: c.Name(), Status: FAIL, Detail: fmt.Sprintf("%s is not writable", path), Hint: "choose writable DB, cache, and run directories"}
		}
		_ = f.Close()
		_ = os.Remove(f.Name())
	}
	if c.paths.db != "" {
		dbPath := filepath.Join(c.paths.db, "foam-clutch-doctor.db")
		st, err := store.Open(dbPath)
		if err != nil {
			return Result{Category: "Storage", Name: c.Name(), Status: FAIL, Detail: "SQLite/WAL could not be opened", Hint: "choose a writable database directory"}
		}
		_ = st.Close()
		_ = os.Remove(dbPath)
		_ = os.Remove(dbPath + "-wal")
		_ = os.Remove(dbPath + "-shm")
	}
	return Result{Category: "Storage", Name: c.Name(), Status: OK, Detail: "DB, cache, and run directories are writable; SQLite WAL is available"}
}

type diskSpaceCheck struct{ path string }

func (diskSpaceCheck) Name() string { return "Disk space" }
func (c diskSpaceCheck) Run(context.Context) Result {
	path := c.path
	if path == "" {
		path = "."
	}
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return Result{Category: "Storage", Name: c.Name(), Status: WARN, Detail: "could not determine free disk space", Hint: "run `df -h " + path + "` and verify available space"}
	}
	free := uint64(stat.Bavail) * uint64(stat.Bsize)
	if free < failFreeBytes {
		return Result{Category: "Storage", Name: c.Name(), Status: FAIL, Detail: fmt.Sprintf("only %s free", formatBytes(free)), Hint: "free disk space before submitting jobs"}
	}
	if free < warnFreeBytes {
		return Result{Category: "Storage", Name: c.Name(), Status: WARN, Detail: fmt.Sprintf("%s free", formatBytes(free)), Hint: "free disk space or configure cache/run directories on a larger filesystem"}
	}
	return Result{Category: "Storage", Name: c.Name(), Status: OK, Detail: fmt.Sprintf("%s free", formatBytes(free))}
}

func formatBytes(n uint64) string {
	const gib = 1024 * 1024 * 1024
	if n >= gib {
		return fmt.Sprintf("%.1f GiB", float64(n)/gib)
	}
	return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
}
