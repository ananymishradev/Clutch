package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/ananymishradev/foam-clutch/internal/store"
	"github.com/ananymishradev/foam-clutch/internal/supervisor"
	"github.com/ananymishradev/foam-clutch/internal/telemetry"
	"github.com/spf13/cobra"
)

type watchConfig struct {
	DB       string
	ID       string
	SlurmID  int
	Backend  string
	Interval time.Duration
	Output   string
}

func newWatchCommand() *cobra.Command {
	cfg := watchConfig{DB: "foam-clutch.db", Backend: "auto", Interval: 15 * time.Second}
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch a submitted Slurm job",
		Long:  "Poll Slurm and reconcile a submitted job until it reaches a terminal state.",
		Example: `  clutch watch --db foam-clutch.db --id <local-job-id> --slurm <slurm-job-id>
  clutch watch --db foam-clutch.db --id <local-job-id> --slurm <slurm-job-id> --backend cli
  clutch watch --db foam-clutch.db --id <local-job-id> --slurm <slurm-job-id> --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatch(cmd.Context(), cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&cfg.DB, "db", cfg.DB, "SQLite database path")
	flags.StringVar(&cfg.ID, "id", "", "local job ID")
	flags.IntVar(&cfg.SlurmID, "slurm", 0, "Slurm job ID")
	flags.StringVar(&cfg.Backend, "backend", cfg.Backend, "scheduler backend: auto, rest, cli")
	flags.DurationVar(&cfg.Interval, "interval", cfg.Interval, "watch poll interval")
	flags.StringVarP(&cfg.Output, "output", "o", outputFormat, "output format: table or json")
	_ = cmd.RegisterFlagCompletionFunc("backend", cobra.FixedCompletions([]string{"auto", "rest", "cli"}, cobra.ShellCompDirectiveNoFileComp))
	return cmd
}

func runWatch(ctx context.Context, cfg watchConfig, stdout, stderr io.Writer) error {
	if cfg.ID == "" || cfg.SlurmID < 1 {
		return fmt.Errorf("watch requires --id <local-job-id> and --slurm <slurm-job-id>")
	}
	scheduler, backendName, err := selectScheduler(cfg.Backend)
	if err != nil {
		return err
	}
	st, err := store.Open(cfg.DB)
	if err != nil {
		return err
	}
	defer st.Close()
	fmt.Fprintf(stderr, "watch backend=%s id=%s slurm=%d\n", backendName, cfg.ID, cfg.SlurmID)
	w := supervisor.Supervisor{Scheduler: scheduler, Store: st, Metrics: &telemetry.Metrics{}, PollInterval: cfg.Interval}
	if err := w.Watch(ctx, cfg.ID, cfg.SlurmID); err != nil {
		return err
	}
	j, err := st.Get(ctx, cfg.ID)
	if err != nil {
		return err
	}
	if cfg.Output == "json" {
		return json.NewEncoder(stdout).Encode(j)
	}
	fmt.Fprintf(stderr, "%s state=%s slurm=%d\n", time.Now().Format(time.RFC3339), j.State, j.SlurmID)
	return nil
}
