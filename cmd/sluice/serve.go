package main

import (
	"context"
	"io"

	"github.com/spf13/cobra"
)

type serveConfig struct {
	Addr string
	DB   string
}

func newServeCommand() *cobra.Command {
	cfg := serveConfig{Addr: ":8080", DB: "foam-clutch.db"}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Foam Clutch HTTP service",
		Long:  "Serve health, readiness, metrics, version, and job endpoints over HTTP.",
		Example: `  clutch serve
  clutch serve --addr :9090
  clutch serve --db /var/lib/foam-clutch/foam-clutch.db`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd.Context(), cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&cfg.Addr, "addr", cfg.Addr, "listen address")
	cmd.Flags().StringVar(&cfg.DB, "db", cfg.DB, "SQLite database path")
	return cmd
}

func runServe(ctx context.Context, cfg serveConfig, stdout, stderr io.Writer) error {
	return serveWithConfig(ctx, cfg.Addr, cfg.DB, stderr)
}
