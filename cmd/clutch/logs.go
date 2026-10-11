package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ananymishradev/foam-clutch/internal/store"
	"github.com/spf13/cobra"
)

type logsConfig struct {
	Manifest string
	DB       string
	ID       string
	Latest   bool
	Follow   bool
	Interval time.Duration
}

func newLogsCommand() *cobra.Command {
	cfg := logsConfig{Follow: true, Interval: 5 * time.Second}
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show or follow job log paths",
		Long:  "Resolve the solver and Slurm log files for a job, or follow them until interrupted.",
		Example: `  clutch logs --manifest case/manifest.yaml --latest
  clutch logs --db foam-clutch.db --id <local-job-id>
  tail -f $(clutch logs --manifest case/manifest.yaml --latest --follow=false)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(cmd.Context(), cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&cfg.Manifest, "manifest", "m", "", "manifest path (sets default database path)")
	flags.StringVar(&cfg.DB, "db", "", "SQLite database path")
	flags.StringVar(&cfg.ID, "id", "", "local job ID")
	flags.BoolVar(&cfg.Latest, "latest", false, "use the most recently submitted job")
	flags.BoolVar(&cfg.Follow, "follow", cfg.Follow, "stream new lines; false prints paths only")
	flags.DurationVar(&cfg.Interval, "interval", cfg.Interval, "stream poll interval")
	_ = cmd.RegisterFlagCompletionFunc("manifest", cobra.FixedCompletions(nil, cobra.ShellCompDirectiveFilterFileExt))
	return cmd
}

func runLogs(ctx context.Context, cfg logsConfig, stdout, stderr io.Writer) error {
	db := strings.TrimSpace(cfg.DB)
	if db == "" {
		if cfg.Manifest != "" {
			db = filepath.Join(filepath.Dir(cfg.Manifest), "foam-clutch.db")
		} else {
			db = "foam-clutch.db"
		}
	}
	if cfg.ID == "" && !cfg.Latest {
		return fmt.Errorf("logs requires --id <local-job-id> or --latest")
	}
	st, err := store.Open(db)
	if err != nil {
		return err
	}
	defer st.Close()
	var j store.Job
	if cfg.Latest {
		jobs, err := st.List(ctx)
		if err != nil {
			return err
		}
		if len(jobs) == 0 {
			return fmt.Errorf("no jobs in %s", db)
		}
		j = jobs[0]
	} else {
		j, err = st.Get(ctx, cfg.ID)
		if err != nil {
			return err
		}
	}
	paths := logPaths(j)
	if !cfg.Follow {
		for _, path := range paths {
			fmt.Fprintln(stdout, path)
		}
		return nil
	}
	for _, path := range paths {
		fmt.Fprintf(stderr, "log: %s\n", path)
	}
	wctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	tailRunLogsTo(wctx, j.RunPath, j.SlurmID, cfg.Interval, stderr)
	return nil
}
