package doctor

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoragePathsCheck(t *testing.T) {
	root := t.TempDir()
	got := (storagePathsCheck{paths: storagePaths{
		db: filepath.Join(root, "db"), cache: filepath.Join(root, "cache"), run: filepath.Join(root, "run"),
	}}).Run(context.Background())
	if got.Status != OK {
		t.Fatalf("result: %#v", got)
	}
}
