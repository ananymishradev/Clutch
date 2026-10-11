package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ananymishradev/sluice/internal/brand"
	"github.com/ananymishradev/sluice/internal/store"
	"github.com/ananymishradev/sluice/internal/supervisor"
	"github.com/ananymishradev/sluice/internal/ui"
	"github.com/spf13/cobra"
)

type statusConfig struct {
	DB      string
	ID      string
	SlurmID int
	Backend string
	Refresh bool
	Output  string
}

func newStatusCommand() *cobra.Command {
	cfg := statusConfig{DB: brand.DefaultDBFile, Backend: "auto"}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the status of a submitted job",
		Long:  "Read a local job record and optionally refresh it from Slurm.",
		Example: "  " + brand.Name + " status --db " + brand.DefaultDBFile + " --id <local-job-id>\n" +
			"  " + brand.Name + " status --db " + brand.DefaultDBFile + " --id <local-job-id> --refresh\n" +
			"  " + brand.Name + " status --db " + brand.DefaultDBFile + " --id <local-job-id> --refresh --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd.Context(), cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&cfg.DB, "db", cfg.DB, "SQLite database path")
	flags.StringVar(&cfg.ID, "id", "", "local job ID")
	flags.IntVar(&cfg.SlurmID, "slurm", 0, "Slurm job ID")
	flags.StringVar(&cfg.Backend, "backend", cfg.Backend, "scheduler backend: auto, rest, cli")
	flags.BoolVar(&cfg.Refresh, "refresh", false, "query Slurm once and update the local record")
	flags.StringVarP(&cfg.Output, "output", "o", outputFormat, "output format: table or json")
	_ = cmd.RegisterFlagCompletionFunc("backend", cobra.FixedCompletions([]string{"auto", "rest", "cli"}, cobra.ShellCompDirectiveNoFileComp))
	return cmd
}

func runStatus(ctx context.Context, cfg statusConfig, stdout, stderr io.Writer) error {
	if cfg.ID == "" {
		return fmt.Errorf("status requires --id <local-job-id>")
	}
	st, err := store.Open(cfg.DB)
	if err != nil {
		return err
	}
	defer st.Close()
	if cfg.Refresh {
		if cfg.SlurmID < 1 {
			j, err := st.Get(ctx, cfg.ID)
			if err != nil {
				return err
			}
			cfg.SlurmID = j.SlurmID
		}
		if cfg.SlurmID < 1 {
			return fmt.Errorf("no Slurm ID known for job %s (not submitted yet?)", cfg.ID)
		}
		scheduler, _, err := selectScheduler(cfg.Backend)
		if err != nil {
			return err
		}
		states, err := scheduler.State(ctx, cfg.SlurmID)
		if err != nil {
			_ = st.UpdateState(ctx, cfg.ID, "UNKNOWN", err.Error(), cfg.SlurmID)
			return fmt.Errorf("slurm state: %w", err)
		}
		if len(states) > 0 {
			_ = st.UpdateState(ctx, cfg.ID, supervisor.ToLocal(states[0]), "", cfg.SlurmID)
		}
	}
	j, err := st.Get(ctx, cfg.ID)
	if err != nil {
		return err
	}
	if strings.EqualFold(cfg.Output, "json") {
		return json.NewEncoder(stdout).Encode(j)
	}
	t := ui.Detect(noColor)
	state := j.State
	if t.Color {
		state = ui.State(j.State).Render(state)
	}
	fmt.Fprintf(stderr, "  %-10s %s\n  %-10s %s\n  %-10s %d\n  %-10s %s\n  %-10s %s\n", "local ID", j.ID, "state", state, "Slurm ID", j.SlurmID, "updated", ui.Relative(j.UpdatedAt), "last error", j.Error)
	return nil
}
