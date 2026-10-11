package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ananymishradev/foam-clutch/internal/doctor"
	"github.com/ananymishradev/foam-clutch/internal/preflight"
	"github.com/ananymishradev/foam-clutch/internal/service"
	"github.com/ananymishradev/foam-clutch/internal/store"
	"github.com/ananymishradev/foam-clutch/internal/ui"
	"github.com/spf13/cobra"
)

const (
	exitOK = iota
	exitJob
	exitUsage
	exitEnvironment
)

var (
	outputFormat string
	noColor      bool
)

type cliObserver struct{}

func (cliObserver) Stage(name string, status service.StageStatus, detail string) {
	if status != service.StageDone {
		return
	}
	t := ui.Detect(noColor)
	ui.Progress{Out: os.Stderr, Term: t, Symbol: ui.SymbolsFor(t)}.Timestamped(name, detail)
}

var legacyFlags = map[string]bool{
	"manifest": true, "db": true, "backend": true, "tail": true, "watch": true,
	"latest": true, "id": true, "slurm": true, "state": true, "refresh": true,
	"follow": true, "interval": true, "timeout": true, "addr": true,
}

type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }
func (e exitError) Unwrap() error { return e.err }

func executeRoot() int {
	os.Args = rewriteLegacyArgs(os.Args)
	root := newRoot()
	if err := root.Execute(); err != nil {
		t := ui.Detect(noColor)
		ui.Error(os.Stderr, "command failed", err, "run `clutch --help` to see available commands", t.Color)
		var coded exitError
		if errors.As(err, &coded) {
			return coded.code
		}
		return exitUsage
	}
	return exitOK
}

func rewriteLegacyArgs(args []string) []string {
	out := append([]string(nil), args...)
	changed := false
	for i := 1; i < len(out); i++ {
		if strings.HasPrefix(out[i], "-") && !strings.HasPrefix(out[i], "--") {
			name := strings.TrimPrefix(out[i], "-")
			if legacyFlags[name] {
				out[i] = "--" + name
				changed = true
			}
		}
	}
	if changed && isTTY(os.Stderr) {
		fmt.Fprintln(os.Stderr, "note: single-dash long flags are deprecated; use --double-dash flags")
	}
	if len(out) > 1 && (out[1] == "--manifest" || out[1] == "--db" || out[1] == "--backend" || out[1] == "--tail") {
		out = append([]string{out[0], "run"}, out[1:]...)
	}
	return out
}

func isTTY(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "clutch",
		Short:         "Run OpenFOAM cases on Slurm",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "output format: table or json")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable terminal colors")
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		t := ui.Detect(noColor)
		if t.TTY {
			fmt.Fprint(os.Stderr, ui.Banner(Version, t.Color))
		}
		_ = cmd.Usage()
	})

	root.AddCommand(
		passthrough("validate", validate),
		passthrough("submit", submit),
		passthrough("watch", watch),
		passthrough("status", status),
		passthrough("cancel", cancel),
		passthrough("logs", logs),
		passthrough("serve", serve),
		newRunCommand(),
		newListCommand(),
		newDoctorCommand(),
		&cobra.Command{Use: "init", DisableFlagParsing: true, RunE: func(cmd *cobra.Command, args []string) error {
			t := ui.Detect(noColor)
			if t.TTY {
				fmt.Fprint(os.Stderr, ui.Banner(Version, t.Color))
			}
			return initManifest(args)
		}},
		&cobra.Command{Use: "version", Run: func(cmd *cobra.Command, args []string) {
			t := ui.Detect(noColor)
			if t.TTY {
				fmt.Fprint(os.Stderr, ui.Banner(Version, t.Color))
			}
			fmt.Printf("clutch %s\n", Version)
		}},
	)
	return root
}

func passthrough(name string, fn func(context.Context, []string) error) *cobra.Command {
	return &cobra.Command{
		Use: name, DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return fn(ctx, args)
		},
	}
}

func newRunCommand() *cobra.Command {
	return &cobra.Command{
		Use: "run", Aliases: []string{"it"}, DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return run(ctx, args)
		},
	}
}

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use: "list", DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error { return listCommand(args) },
	}
}

func listCommand(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&outputFormat, "o", outputFormat, "output format: table or json")
	fs.StringVar(&outputFormat, "output", outputFormat, "output format: table or json")
	dbPath := fs.String("db", "foam-clutch.db", "SQLite database path")
	state := fs.String("state", "", "filter by state")
	limit := fs.Int("limit", 50, "maximum number of jobs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	jobs, err := st.ListByState(context.Background(), strings.ToUpper(strings.TrimSpace(*state)))
	if err != nil {
		return err
	}
	if *limit >= 0 && len(jobs) > *limit {
		jobs = jobs[:*limit]
	}
	if strings.EqualFold(outputFormat, "json") {
		rows := make([]map[string]any, 0, len(jobs))
		for _, j := range jobs {
			rows = append(rows, ui.JobJSON(j))
		}
		return json.NewEncoder(os.Stdout).Encode(rows)
	}
	t := ui.Detect(noColor)
	if len(jobs) == 0 {
		fmt.Fprintln(os.Stderr, "No jobs yet. Try: clutch run --manifest manifest.yaml")
		return nil
	}
	ui.JobsTable(os.Stderr, jobs, t.Color)
	return nil
}

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use: "doctor", DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error { return doctorCommand(args) },
	}
}

func doctorCommand(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&outputFormat, "o", outputFormat, "output format: table or json")
	fs.StringVar(&outputFormat, "output", outputFormat, "output format: table or json")
	timeout := fs.Duration("timeout", 5*time.Second, "per-check timeout")
	dbPath := fs.String("db", "foam-clutch.db", "SQLite database path")
	manifestPath := fs.String("manifest", "", "optional manifest to validate")
	if err := fs.Parse(args); err != nil {
		return err
	}
	checks := doctor.DefaultChecks(*dbPath)
	if strings.TrimSpace(*manifestPath) != "" {
		checks = append(checks, caseDoctorCheck{manifestPath: *manifestPath})
	}
	results := doctor.Runner{Timeout: *timeout, Checks: checks}.Run(context.Background())
	if strings.EqualFold(outputFormat, "json") {
		return json.NewEncoder(os.Stdout).Encode(results)
	}
	t := ui.Detect(noColor)
	counts := map[doctor.Status]int{}
	for _, result := range results {
		counts[result.Status]++
		symbol := ui.SymbolsFor(t).OK
		switch result.Status {
		case doctor.WARN:
			symbol = ui.SymbolsFor(t).Warn
		case doctor.FAIL:
			symbol = ui.SymbolsFor(t).Fail
		case doctor.SKIP:
			symbol = ui.SymbolsFor(t).Skip
		}
		fmt.Fprintf(os.Stderr, "%s %-18s %s\n", symbol, result.Name, result.Detail)
		if result.Hint != "" {
			fmt.Fprintf(os.Stderr, "  hint: %s\n", result.Hint)
		}
	}
	fmt.Fprintf(os.Stderr, "\n%d passed · %d warning · %d failed\n", counts[doctor.OK], counts[doctor.WARN], counts[doctor.FAIL])
	if counts[doctor.FAIL] > 0 {
		return exitError{code: exitEnvironment, err: fmt.Errorf("doctor found environment failures")}
	}
	return nil
}

type caseDoctorCheck struct{ manifestPath string }

func (caseDoctorCheck) Name() string { return "Case" }
func (c caseDoctorCheck) Run(ctx context.Context) doctor.Result {
	s := service.Service{Preflight: preflight.Runner{}}
	m, _, err := s.Validate(ctx, c.manifestPath)
	if err != nil {
		return doctor.Result{Category: "Case", Name: "Case", Status: doctor.FAIL, Detail: err.Error(), Hint: "fix the manifest or preflight findings before submitting"}
	}
	return doctor.Result{Category: "Case", Name: "Case", Status: doctor.OK, Detail: fmt.Sprintf("%s is valid", m.Name)}
}
