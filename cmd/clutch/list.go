package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ananymishradev/foam-clutch/internal/store"
	"github.com/ananymishradev/foam-clutch/internal/ui"
	"github.com/spf13/cobra"
)

type listConfig struct {
	DB     string
	State  string
	Limit  int
	Output string
}

func newListCommand() *cobra.Command {
	cfg := listConfig{DB: "foam-clutch.db", Limit: 50}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List submitted jobs",
		Long:  "List local Foam Clutch jobs, optionally filtered by state.",
		Example: `  clutch list
  clutch list --state RUNNING --limit 20
  clutch list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&cfg.DB, "db", cfg.DB, "SQLite database path")
	cmd.Flags().StringVar(&cfg.State, "state", "", "filter by state")
	cmd.Flags().IntVar(&cfg.Limit, "limit", cfg.Limit, "maximum number of jobs")
	cmd.Flags().StringVarP(&cfg.Output, "output", "o", outputFormat, "output format: table or json")
	return cmd
}

func runList(ctx context.Context, cfg listConfig, stdout, stderr io.Writer) error {
	st, err := store.Open(cfg.DB)
	if err != nil {
		return err
	}
	defer st.Close()
	jobs, err := st.ListByState(ctx, strings.ToUpper(strings.TrimSpace(cfg.State)))
	if err != nil {
		return err
	}
	if cfg.Limit >= 0 && len(jobs) > cfg.Limit {
		jobs = jobs[:cfg.Limit]
	}
	if strings.EqualFold(cfg.Output, "json") {
		rows := make([]map[string]any, 0, len(jobs))
		for _, j := range jobs {
			rows = append(rows, ui.JobJSON(j))
		}
		return json.NewEncoder(stdout).Encode(rows)
	}
	if len(jobs) == 0 {
		fmt.Fprintln(stderr, "No jobs yet. Try: clutch run --manifest manifest.yaml")
		return nil
	}
	ui.JobsTable(stderr, jobs, ui.Detect(noColor).Color)
	return nil
}
