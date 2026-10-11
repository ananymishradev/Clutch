package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ananymishradev/foam-clutch/internal/manifest"
	"github.com/ananymishradev/foam-clutch/internal/preflight"
	"github.com/ananymishradev/foam-clutch/internal/service"
	"github.com/ananymishradev/foam-clutch/internal/store"
	"github.com/ananymishradev/foam-clutch/internal/supervisor"
	"github.com/ananymishradev/foam-clutch/internal/telemetry"
	"github.com/ananymishradev/foam-clutch/internal/ui"
	"github.com/ananymishradev/foam-clutch/internal/ui/runview"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type liveRunOptions struct {
	Manifest, DB, Backend           string
	Timeout, Interval, TailInterval time.Duration
	Tail, NoWatch                   bool
}

func detachRun(isSubmitted func() bool, stopPipeline, stopWatch func()) {
	if isSubmitted() {
		stopWatch()
		return
	}
	stopPipeline()
}

func liveRunEnabled() bool {
	return term.IsTerminal(int(os.Stderr.Fd())) && !noColor &&
		os.Getenv("NO_COLOR") == "" && !strings.EqualFold(outputFormat, "json")
}

func runLive(ctx context.Context, opts liveRunOptions) error {
	scheduler, _, err := selectScheduler(opts.Backend)
	if err != nil {
		return err
	}
	st, err := store.Open(opts.DB)
	if err != nil {
		return err
	}
	defer st.Close()

	runCtx, stopRun := context.WithCancel(ctx)
	defer stopRun()
	watchCtx, stopWatch := context.WithCancel(runCtx)
	defer stopWatch()
	var submitted atomic.Bool
	var result store.Job
	var submitResult service.Result
	var submitErr error
	resultCh := make(chan struct{})

	var program *tea.Program
	onDetach := func() {
		detachRun(submitted.Load, stopRun, stopWatch)
	}
	checkpointSeconds := 0
	if m, loadErr := manifest.Load(opts.Manifest); loadErr == nil {
		checkpointSeconds = m.Solver.CheckpointSeconds
	}

	model := runview.New([]string{"manifest", "cache", "run dir", "submitted"}, checkpointSeconds, true, onDetach)
	program = tea.NewProgram(model, tea.WithOutput(os.Stderr))

	go func() {
		defer close(resultCh)
		observer := liveStageObserver{send: program.Send}
		metrics := &telemetry.Metrics{}
		s := service.Service{Scheduler: scheduler, Store: st, Preflight: preflight.Runner{}, Metrics: metrics, Observer: observer}
		submitCtx := runCtx
		if opts.Timeout > 0 {
			var cancel context.CancelFunc
			submitCtx, cancel = context.WithTimeout(runCtx, opts.Timeout)
			defer cancel()
		}
		submitResult, submitErr = s.Submit(submitCtx, opts.Manifest)
		if submitErr != nil {
			program.Send(runview.DoneMsg{State: "FAILED", Err: submitErr})
			return
		}
		submitted.Store(true)
		result = store.Job{ID: submitResult.ID, SlurmID: submitResult.SlurmID, RunPath: submitResult.RunPath, CachePath: submitResult.CachePath, CaseHash: submitResult.CaseHash, State: "QUEUED"}
		program.Send(runview.StateMsg{State: "QUEUED", SlurmID: submitResult.SlurmID})
		if opts.NoWatch {
			program.Send(runview.DoneMsg{State: "QUEUED"})
			return
		}
		if opts.Tail {
			go tailSolverToProgram(watchCtx, program, submitResult.RunPath, opts.TailInterval)
		}
		w := supervisor.Supervisor{
			Scheduler: scheduler, Store: st, Metrics: metrics, PollInterval: opts.Interval,
			Observer: func(state string, slurmID int) { program.Send(runview.StateMsg{State: state, SlurmID: slurmID}) },
		}
		err := w.Watch(watchCtx, submitResult.ID, submitResult.SlurmID)
		j, getErr := st.Get(context.Background(), submitResult.ID)
		if getErr == nil {
			result = j
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			submitErr = err
			program.Send(runview.DoneMsg{State: result.State, Err: err})
			return
		}
		program.Send(runview.DoneMsg{State: result.State})
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			program.Send(runview.DetachMsg{})
		case <-resultCh:
		case <-ctx.Done():
		}
	}()
	finalTeaModel, err := program.Run()
	if err != nil {
		return err
	}
	finalModel, ok := finalTeaModel.(runview.Model)
	if !ok {
		return fmt.Errorf("live view returned unexpected model type")
	}
	<-resultCh
	if !submitted.Load() {
		if finalModel.Detached {
			return exitError{code: 130, err: fmt.Errorf("interrupted before submission")}
		}
		if submitErr != nil {
			return submitErr
		}
	}
	if finalModel.Detached {
		fmt.Fprintf(os.Stderr, "job still running · local %s · slurm %d\nstatus:  clutch status --id %s --refresh\ncancel:  clutch cancel --id %s\n", submitResult.ID, submitResult.SlurmID, submitResult.ID, submitResult.ID)
		return nil
	}
	if submitErr != nil {
		return exitError{code: 1, err: submitErr}
	}
	return liveSummary(result, finalModel.FinalState, finalModel.Now.Sub(finalModel.Started), finalModel.Err)
}

type liveStageObserver struct{ send func(tea.Msg) }

func (o liveStageObserver) Stage(name string, status service.StageStatus, detail string) {
	o.send(runview.StageMsg{Name: name, Status: string(status), Detail: detail})
}

func tailSolverToProgram(ctx context.Context, p *tea.Program, runPath string, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	var offset int64
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f, err := os.Open(filepath.Join(runPath, "log.solver"))
			if err != nil {
				continue
			}
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				f.Close()
				continue
			}
			data, err := io.ReadAll(io.LimitReader(f, 1<<20))
			f.Close()
			if err != nil {
				continue
			}
			offset += int64(len(data))
			for _, line := range strings.Split(string(data), "\n") {
				if line != "" {
					p.Send(runview.LogLineMsg{Line: line})
				}
			}
		}
	}
}

func liveSummary(j store.Job, state string, elapsed time.Duration, runErr error) error {
	if state == "" {
		state = j.State
	}
	if elapsed < 0 {
		elapsed = 0
	}
	logs := logPaths(j)
	stateText := ui.State(state).Render(state)
	body := fmt.Sprintf("state:    %s\nelapsed:  %s\nlocal ID: %s\nSlurm ID: %d\nrun dir:  %s\nlogs:\n  %s\n  %s\n  %s",
		stateText, ui.Duration(elapsed), j.ID, j.SlurmID, j.RunPath, logs[0], logs[1], logs[2])
	if state == "FAILED" {
		lastError := j.Error
		if lastError == "" && runErr != nil {
			lastError = runErr.Error()
		}
		body += fmt.Sprintf("\nlast error: %s\nhint: tail -f %s", lastError, logs[0])
	}
	fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Render(body))
	if state == "FAILED" || state == "CANCELLED" {
		return exitError{code: 1, err: fmt.Errorf("job ended in %s", state)}
	}
	return nil
}
