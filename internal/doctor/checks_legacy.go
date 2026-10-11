package doctor

import (
	"context"
	"os/exec"
	"strings"

	"github.com/ananymishradev/sluice/internal/brand"
)

// legacyInstallCheck warns when another clutch binary identifying as
// Foam Clutch is on PATH. Identification uses version output, never name alone.
type legacyInstallCheck struct {
	lookPath func(string) (string, error)
	run      func(path string) (string, error)
}

func (legacyInstallCheck) Name() string { return "Legacy install" }

func (c legacyInstallCheck) Run(context.Context) Result {
	look := c.lookPath
	if look == nil {
		look = exec.LookPath
	}
	path, err := look(brand.LegacyName)
	if err != nil {
		return Result{Category: "Install", Name: c.Name(), Status: OK, Detail: "no legacy " + brand.LegacyName + " on PATH"}
	}
	run := c.run
	if run == nil {
		run = func(p string) (string, error) {
			out, err := exec.Command(p, "version").CombinedOutput()
			return string(out), err
		}
	}
	out, err := run(path)
	if err != nil {
		return Result{Category: "Install", Name: c.Name(), Status: OK, Detail: "legacy binary at " + path + " did not respond as " + brand.LegacyDisplayName}
	}
	if strings.Contains(out, brand.LegacyDisplayName) || strings.Contains(out, brand.LegacyAPIVersion) {
		return Result{Category: "Install", Name: c.Name(), Status: WARN, Detail: "legacy " + brand.LegacyDisplayName + " install found at " + path, Hint: "remove it after migrating to " + brand.Name + " (see Migrating from " + brand.LegacyDisplayName + " in README)"}
	}
	return Result{Category: "Install", Name: c.Name(), Status: OK, Detail: "binary named " + brand.LegacyName + " at " + path + " is unrelated"}
}
