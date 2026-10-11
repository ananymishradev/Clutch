package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/ananymishradev/sluice/internal/brand"
	"github.com/ananymishradev/sluice/internal/doctor"
	"github.com/ananymishradev/sluice/internal/manifest"
	"github.com/ananymishradev/sluice/internal/preflight"
	"github.com/ananymishradev/sluice/internal/service"
	"github.com/ananymishradev/sluice/internal/ui"
	"github.com/spf13/cobra"
)

type doctorConfig struct {
	DB       string
	Manifest string
	Timeout  time.Duration
	Output   string
}

func newDoctorCommand() *cobra.Command {
	cfg := doctorConfig{DB: brand.DefaultDBFile, Timeout: 5 * time.Second}
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the local " + brand.DisplayName + " environment",
		Long:  "Check storage, Slurm, REST, JWT, and OpenFOAM prerequisites.",
		Example: "  " + brand.Name + " doctor\n" +
			"  " + brand.Name + " doctor --manifest manifest.yaml\n" +
			"  " + brand.Name + " doctor --timeout 10s --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context(), cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&cfg.DB, "db", cfg.DB, "SQLite database path")
	cmd.Flags().StringVarP(&cfg.Manifest, "manifest", "m", "", "optional manifest to validate")
	cmd.Flags().DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "per-check timeout")
	cmd.Flags().StringVarP(&cfg.Output, "output", "o", outputFormat, "output format: table or json")
	return cmd
}

func runDoctor(ctx context.Context, cfg doctorConfig, stdout, stderr io.Writer) error {
	cacheDir := filepath.Join(filepath.Dir(cfg.DB), ".foam-cache")
	runDir := filepath.Join(filepath.Dir(cfg.DB), ".foam-runs")
	checkpointSeconds, foamBashrc := 0, ""
	if strings.TrimSpace(cfg.Manifest) != "" {
		m, err := manifest.Load(cfg.Manifest)
		if err == nil {
			cacheDir, runDir = m.Storage.CacheDir, m.Storage.RunDir
			if cacheDir == "" {
				cacheDir = filepath.Join(filepath.Dir(cfg.Manifest), ".foam-cache")
			}
			if runDir == "" {
				runDir = filepath.Join(filepath.Dir(cfg.Manifest), ".foam-runs")
			}
			checkpointSeconds, foamBashrc = m.Solver.CheckpointSeconds, m.Runtime.FoamBashrc
		}
	}
	checks := doctor.DefaultChecksFor(cfg.DB, cacheDir, runDir, checkpointSeconds, foamBashrc)
	if cfg.Manifest != "" {
		checks = append(checks, caseDoctorCheck{manifestPath: cfg.Manifest})
	}
	results := doctor.Runner{Timeout: cfg.Timeout, Checks: checks}.Run(ctx)
	if strings.EqualFold(cfg.Output, "json") {
		return json.NewEncoder(stdout).Encode(results)
	}
	t := ui.Detect(noColor)
	counts := map[doctor.Status]int{}
	for _, result := range results {
		counts[result.Status]++
		symbol := ui.SymbolsFor(t).OK
		switch result.Status {
		case doctor.WARN:
			symbol = ui.SymbolsFor(t).Warn
		case doctor.FAIL:
			symbol = ui.SymbolsFor(t).Fail
		case doctor.SKIP:
			symbol = ui.SymbolsFor(t).Skip
		}
		fmt.Fprintf(stderr, "%s %-18s %s\n", symbol, result.Name, result.Detail)
		if result.Hint != "" {
			fmt.Fprintf(stderr, "  hint: %s\n", result.Hint)
		}
	}
	fmt.Fprintf(stderr, "\n%d passed · %d warning · %d failed\n", counts[doctor.OK], counts[doctor.WARN], counts[doctor.FAIL])
	if counts[doctor.FAIL] > 0 {
		return exitError{code: exitEnvironment, err: fmt.Errorf("doctor found environment failures")}
	}
	return nil
}

type caseDoctorCheck struct{ manifestPath string }

func (caseDoctorCheck) Name() string { return "Case" }
func (c caseDoctorCheck) Run(ctx context.Context) doctor.Result {
	s := service.Service{Preflight: preflight.Runner{}}
	m, _, err := s.Validate(ctx, c.manifestPath)
	if err != nil {
		return doctor.Result{Category: "Case", Name: "Case", Status: doctor.FAIL, Detail: err.Error(), Hint: "fix the manifest or preflight findings before submitting"}
	}
	return doctor.Result{Category: "Case", Name: "Case", Status: doctor.OK, Detail: fmt.Sprintf("%s is valid", m.Name)}
}
