package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ananymishradev/foam-clutch/internal/preflight"
	"github.com/ananymishradev/foam-clutch/internal/service"
	"github.com/ananymishradev/foam-clutch/internal/store"
	"github.com/ananymishradev/foam-clutch/internal/supervisor"
	"github.com/ananymishradev/foam-clutch/internal/telemetry"
	"github.com/spf13/cobra"
)

type submitConfig struct {
	Manifest string
	DB       string
	Watch    bool
	Backend  string
	Timeout  time.Duration
	Interval time.Duration
	Output   string
}

func newSubmitCommand() *cobra.Command {
	cfg := submitConfig{Manifest: "manifest.yaml", DB: "foam-clutch.db", Backend: "auto", Timeout: 5 * time.Minute, Interval: 15 * time.Second}
	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Submit an OpenFOAM case to Slurm",
		Long:  "Validate, stage, and submit an OpenFOAM case. Use --watch to wait for a terminal Slurm state.",
		Example: `  clutch submit --manifest manifest.yaml --backend cli
  clutch submit --manifest manifest.yaml --db foam-clutch.db --watch
  clutch submit --manifest manifest.yaml --backend rest --timeout 2m`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSubmit(cmd.Context(), cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&cfg.Manifest, "manifest", "m", cfg.Manifest, "path to manifest")
	flags.StringVar(&cfg.DB, "db", cfg.DB, "SQLite database path")
	flags.BoolVar(&cfg.Watch, "watch", false, "watch Slurm state until terminal")
	flags.StringVar(&cfg.Backend, "backend", cfg.Backend, "scheduler backend: auto, rest, cli")
	flags.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "submit timeout (0 disables)")
	flags.DurationVar(&cfg.Interval, "interval", cfg.Interval, "watch poll interval")
	_ = cmd.RegisterFlagCompletionFunc("backend", cobra.FixedCompletions([]string{"auto", "rest", "cli"}, cobra.ShellCompDirectiveNoFileComp))
	_ = cmd.RegisterFlagCompletionFunc("manifest", cobra.FixedCompletions([]string{"manifest.yaml", "examples/generic.yaml"}, cobra.ShellCompDirectiveDefault))
	return cmd
}

func runSubmit(ctx context.Context, cfg submitConfig, stdout, stderr io.Writer) error {
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	scheduler, backendName, err := selectScheduler(cfg.Backend)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "submit backend=%s manifest=%s\n", backendName, cfg.Manifest)
	st, err := store.Open(cfg.DB)
	if err != nil {
		return err
	}
	defer st.Close()
	metrics := &telemetry.Metrics{}
	s := service.Service{Scheduler: scheduler, Store: st, Preflight: preflight.Runner{}, Metrics: metrics}
	result, err := s.Submit(ctx, cfg.Manifest)
	if err != nil {
		return err
	}
	if cfg.Output == "json" {
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			return err
		}
	} else {
		writeSubmitSummary(stderr, result.ID, result.SlurmID, result.CaseHash, result.RunPath, result.CachePath)
	}
	if !cfg.Watch {
		return nil
	}
	wctx := context.WithoutCancel(ctx)
	wctx, stop := signal.NotifyContext(wctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	w := supervisor.Supervisor{Scheduler: scheduler, Store: st, Metrics: metrics, PollInterval: cfg.Interval}
	return w.Watch(wctx, result.ID, result.SlurmID)
}

func writeSubmitSummary(w io.Writer, id string, slurmID int, hash, runPath, cachePath string) {
	fmt.Fprintf(w, "  %-10s %s\n  %-10s %d\n  %-10s %s\n  %-10s %s\n  %-10s %s\n", "local ID", id, "Slurm ID", slurmID, "case hash", hash, "run path", runPath, "cache path", cachePath)
}
