package main

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"

	"github.com/ananymishradev/sluice/internal/brand"
	"github.com/spf13/cobra"
)

var (
	Commit    = "unknown"
	BuildDate = "unknown"
)

func newVersionCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Print the " + brand.DisplayName + " version, commit, build date, and Go version.",
		Example: "  " + brand.Name + " version\n" +
			"  " + brand.Name + " version --output json\n" +
			"  " + brand.Name + " version --no-color",
		RunE: func(cmd *cobra.Command, args []string) error {
			if output != "json" {
				fmt.Fprint(cmd.ErrOrStderr(), bannerForCommand())
			}
			return runVersion(output, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", outputFormat, "output format: table or json")
	return cmd
}

func runVersion(output string, stdout, stderr io.Writer) error {
	info := map[string]string{"version": Version, "commit": Commit, "buildDate": BuildDate, "goVersion": runtime.Version()}
	if output == "json" {
		return json.NewEncoder(stdout).Encode(info)
	}
	fmt.Fprintf(stderr, brand.Name+" %s\ncommit: %s\nbuild date: %s\ngo: %s\n", Version, Commit, BuildDate, runtime.Version())
	return nil
}
