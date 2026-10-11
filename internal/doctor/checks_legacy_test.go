package doctor

import (
	"context"
	"testing"
)

func TestLegacyInstallWarnsOnFoamClutch(t *testing.T) {
	c := legacyInstallCheck{
		lookPath: func(string) (string, error) { return "/bin/clutch", nil },
		run:      func(string) (string, error) { return "sluice dev\nFoam Clutch\n", nil },
	}
	r := c.Run(context.Background())
	if r.Status != WARN || r.Name != "Legacy install" {
		t.Fatalf("got %#v", r)
	}
}

func TestLegacyInstallOKWhenAbsent(t *testing.T) {
	c := legacyInstallCheck{
		lookPath: func(string) (string, error) { return "", context.DeadlineExceeded },
	}
	if r := c.Run(context.Background()); r.Status != OK {
		t.Fatalf("got %#v", r)
	}
}

func TestLegacyInstallOKWhenUnrelated(t *testing.T) {
	c := legacyInstallCheck{
		lookPath: func(string) (string, error) { return "/bin/clutch", nil },
		run:      func(string) (string, error) { return "another tool v1\n", nil },
	}
	if r := c.Run(context.Background()); r.Status != OK {
		t.Fatalf("got %#v", r)
	}
}
