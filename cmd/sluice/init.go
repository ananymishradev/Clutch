package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ananymishradev/sluice/internal/brand"
	"github.com/ananymishradev/sluice/internal/manifest"
	"github.com/ananymishradev/sluice/internal/ui"
	"github.com/spf13/cobra"
)

type initConfig struct {
	Output     string
	Name       string
	Case       string
	Solver     string
	Partition  string
	Nodes      int
	Tasks      int
	Time       int
	Checkpoint int
}

func newInitCommand() *cobra.Command {
	cfg := initConfig{Output: "manifest.yaml", Name: "myCase", Case: "./case", Solver: "foamRun", Nodes: 1, Tasks: 4, Time: 60, Checkpoint: 600}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a starter manifest",
		Long:  "Scaffold a manifest for an OpenFOAM case and Slurm job.",
		Example: "  " + brand.Name + " init --output manifest.yaml --case ./case\n" +
			"  " + brand.Name + " init --solver simpleFoam --nodes 2 --tasks 32\n" +
			"  " + brand.Name + " init --name cavity --checkpoint 0",
		RunE: func(cmd *cobra.Command, args []string) error {
			if ui := cmd.ErrOrStderr(); ui != nil {
				fmt.Fprint(ui, bannerForCommand())
			}
			return runInit(cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&cfg.Output, "output", "o", cfg.Output, "output manifest path")
	flags.StringVar(&cfg.Name, "name", cfg.Name, "Slurm/local job name")
	flags.StringVar(&cfg.Case, "case", cfg.Case, "case directory relative to the manifest")
	flags.StringVar(&cfg.Solver, "solver", cfg.Solver, "solver executable")
	flags.StringVar(&cfg.Partition, "partition", cfg.Partition, "Slurm partition")
	flags.IntVar(&cfg.Nodes, "nodes", cfg.Nodes, "node count")
	flags.IntVar(&cfg.Tasks, "tasks", cfg.Tasks, "tasks per node")
	flags.IntVar(&cfg.Time, "time", cfg.Time, "walltime in minutes")
	flags.IntVar(&cfg.Checkpoint, "checkpoint", cfg.Checkpoint, "checkpoint seconds before walltime; 0 disables")
	return cmd
}

func bannerForCommand() string {
	t := ui.Detect(noColor)
	if t.TTY {
		return ui.Banner(Version, t.Color)
	}
	return ""
}

func runInit(cfg initConfig, stdout, _ io.Writer) error {
	doc := fmt.Sprintf(`apiVersion: `+brand.APIVersion+`
name: %s

case:
  path: %s
  # include: ["Allrun"]  # extra top-level case files to stage

resources:
  partition: %s
  account: ""
  nodes: %d
  tasksPerNode: %d
  timeLimitMinutes: %d

solver:
  name: %s
  arguments: []
  fileHandler: collated
  checkpointSeconds: %d

mesh:
  strategy: auto

post:
  reconstruct: false

runtime:
  foamBashrc: %s
  threadsPerRank: 1
  mpi:
    launcher: mpirun
    flavour: pmix

storage:
  cacheDir: ./.foam-cache
  runDir: ./.foam-runs
`, cfg.Name, cfg.Case, cfg.Partition, cfg.Nodes, cfg.Tasks, cfg.Time, cfg.Solver, cfg.Checkpoint, manifest.DefaultFoamBashrc)
	if err := os.WriteFile(cfg.Output, []byte(doc), 0o640); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stdout, "wrote %s\n", cfg.Output)
	return err
}
