package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ananymishradev/sluice/internal/brand"
	"github.com/ananymishradev/sluice/internal/store"
	"github.com/spf13/cobra"
)

type cancelConfig struct {
	DB      string
	ID      string
	SlurmID int
	Backend string
	Yes     bool
	Output  string
}

func newCancelCommand() *cobra.Command {
	cfg := cancelConfig{DB: brand.DefaultDBFile, Backend: "auto"}
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a submitted Slurm job",
		Long:  "Cancel a local job or Slurm job and update the local record when an ID is available.",
		Example: "  " + brand.Name + " cancel --db " + brand.DefaultDBFile + " --id <local-job-id>\n" +
			"  " + brand.Name + " cancel --db " + brand.DefaultDBFile + " --slurm <slurm-job-id> --yes\n" +
			"  " + brand.Name + " cancel --db " + brand.DefaultDBFile + " --id <local-job-id> --backend rest",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCancel(cmd.Context(), cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&cfg.DB, "db", cfg.DB, "SQLite database path")
	flags.StringVar(&cfg.ID, "id", "", "local job ID")
	flags.IntVar(&cfg.SlurmID, "slurm", 0, "Slurm job ID")
	flags.StringVar(&cfg.Backend, "backend", cfg.Backend, "scheduler backend: auto, rest, cli")
	flags.BoolVar(&cfg.Yes, "yes", false, "skip confirmation")
	flags.StringVarP(&cfg.Output, "output", "o", outputFormat, "output format: table or json")
	_ = cmd.RegisterFlagCompletionFunc("backend", cobra.FixedCompletions([]string{"auto", "rest", "cli"}, cobra.ShellCompDirectiveNoFileComp))
	return cmd
}

func runCancel(ctx context.Context, cfg cancelConfig, stdout, stderr io.Writer) error {
	if cfg.ID == "" && cfg.SlurmID < 1 {
		return fmt.Errorf("cancel requires --id <local-job-id> and/or --slurm <slurm-job-id>")
	}
	st, err := store.Open(cfg.DB)
	if err != nil {
		return err
	}
	defer st.Close()
	if cfg.SlurmID < 1 {
		j, err := st.Get(ctx, cfg.ID)
		if err != nil {
			return err
		}
		cfg.SlurmID = j.SlurmID
	}
	if cfg.SlurmID < 1 {
		return fmt.Errorf("no Slurm ID known (job not submitted?)")
	}
	if !cfg.Yes && isTTY(os.Stdin) && !confirmCancel(cfg.SlurmID, stderr) {
		fmt.Fprintln(stderr, "cancelled")
		return nil
	}
	scheduler, backendName, err := selectScheduler(cfg.Backend)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "cancelling Slurm job %d via %s\n", cfg.SlurmID, backendName)
	if err := scheduler.Cancel(ctx, cfg.SlurmID); err != nil {
		return err
	}
	if cfg.ID != "" {
		_ = st.UpdateState(ctx, cfg.ID, "CANCELLED", "cancelled by operator", cfg.SlurmID)
		j, err := st.Get(ctx, cfg.ID)
		if err != nil {
			return err
		}
		if strings.EqualFold(cfg.Output, "json") {
			return json.NewEncoder(stdout).Encode(j)
		}
		fmt.Fprintf(stderr, "cancelled %s (Slurm %d)\n", j.ID, j.SlurmID)
		return nil
	}
	fmt.Fprintf(stderr, "cancelled Slurm job %d\n", cfg.SlurmID)
	return nil
}

func confirmCancel(slurmID int, stderr io.Writer) bool {
	fmt.Fprintf(stderr, "Cancel Slurm job %d? [y/N] ", slurmID)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return err == nil && strings.EqualFold(strings.TrimSpace(answer), "y")
}
