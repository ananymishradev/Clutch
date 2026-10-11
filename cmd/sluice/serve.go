package main

import (
	"context"
	"io"

	"github.com/ananymishradev/sluice/internal/brand"
	"github.com/spf13/cobra"
)

type serveConfig struct {
	Addr string
	DB   string
}

func newServeCommand() *cobra.Command {
	cfg := serveConfig{Addr: ":8080", DB: brand.DefaultDBFile}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the " + brand.DisplayName + " HTTP service",
		Long:  "Serve health, readiness, metrics, version, and job endpoints over HTTP.",
		Example: "  " + brand.Name + " serve\n" +
			"  " + brand.Name + " serve --addr :9090\n" +
			"  " + brand.Name + " serve --db /var/lib/" + brand.Name + "/" + brand.DefaultDBFile,
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
