package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequeueCheck(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "scontrol-config-enabled.txt"))
	if err != nil {
		t.Fatal(err)
	}
	enabled := (requeueCheck{run: func(context.Context, string, ...string) ([]byte, error) {
		return fixture, nil
	}}).Run(context.Background())
	if enabled.Status != OK {
		t.Fatalf("enabled: %#v", enabled)
	}

	fixture, err = os.ReadFile(filepath.Join("testdata", "scontrol-config-disabled.txt"))
	if err != nil {
		t.Fatal(err)
	}
	disabled := (requeueCheck{
		checkpointSeconds: 600,
		run:               func(context.Context, string, ...string) ([]byte, error) { return fixture, nil },
	}).Run(context.Background())
	if disabled.Status != FAIL || !strings.Contains(disabled.Hint, "checkpoint/requeue will fail") {
		t.Fatalf("disabled: %#v", disabled)
	}
}
