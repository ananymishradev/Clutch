package doctor

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
)

type commandRunner func(context.Context, string, ...string) ([]byte, error)

type requeueCheck struct {
	run               commandRunner
	checkpointSeconds int
}

func (requeueCheck) Name() string { return "Slurm policy" }

func (c requeueCheck) Run(ctx context.Context) Result {
	if c.run == nil {
		c.run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		}
	}
	out, err := c.run(ctx, "scontrol", "show", "config")
	if err != nil {
		return Result{Category: "Slurm", Name: c.Name(), Status: WARN, Detail: "could not determine requeue policy", Hint: "run `scontrol show config` and ask your admin about JobRequeue"}
	}
	match := regexp.MustCompile(`(?m)^\s*JobRequeue\s*=\s*([01])\b`).FindStringSubmatch(string(out))
	if len(match) != 2 {
		return Result{Category: "Slurm", Name: c.Name(), Status: WARN, Detail: "could not determine requeue policy", Hint: "run `scontrol show config` and ask your admin about JobRequeue"}
	}
	if match[1] == "0" {
		result := Result{Category: "Slurm", Name: c.Name(), Status: WARN, Detail: "requeue disabled cluster-wide", Hint: requeueHint}
		if c.checkpointSeconds > 0 {
			result.Status = FAIL
		}
		return result
	}
	return Result{Category: "Slurm", Name: c.Name(), Status: OK, Detail: "requeue enabled"}
}

const requeueHint = "requeue disabled cluster-wide; checkpoint/requeue will fail. Set solver.checkpointSeconds: 0 or ask your admin"

func (c requeueCheck) String() string { return strings.TrimSpace(c.Name()) }
