package doctor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Status string

const (
	OK   Status = "OK"
	WARN Status = "WARN"
	FAIL Status = "FAIL"
	SKIP Status = "SKIP"
)

type Result struct {
	Category string        `json:"category"`
	Name     string        `json:"name"`
	Status   Status        `json:"status"`
	Detail   string        `json:"detail"`
	Hint     string        `json:"hint,omitempty"`
	Elapsed  time.Duration `json:"elapsed"`
}

type Check interface {
	Name() string
	Run(context.Context) Result
}

type Runner struct {
	Timeout time.Duration
	Checks  []Check
}

func (r Runner) Run(ctx context.Context) []Result {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	results := make([]Result, len(r.Checks))
	var wg sync.WaitGroup
	for i, check := range r.Checks {
		wg.Add(1)
		go func(i int, check Check) {
			defer wg.Done()
			start := time.Now()
			cctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			result := check.Run(cctx)
			if result.Name == "" {
				result.Name = check.Name()
			}
			result.Elapsed = time.Since(start).Round(time.Millisecond)
			results[i] = result
		}(i, check)
	}
	wg.Wait()
	return results
}

type commandCheck struct {
	category string
	name     string
	commands []string
}

func (c commandCheck) Name() string { return c.name }
func (c commandCheck) Run(ctx context.Context) Result {
	missing := []string{}
	for _, name := range c.commands {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return Result{Category: c.category, Name: c.name, Status: OK, Detail: strings.Join(c.commands, ", ") + " available"}
	}
	status := FAIL
	hint := "install Slurm and add its commands to PATH"
	if c.name == "Slurm CLI" && len(missing) == 1 && missing[0] == "sacct" {
		status = WARN
		hint = "install Slurm accounting tools for complete job-state history"
	}
	return Result{Category: c.category, Name: c.name, Status: status, Detail: "missing: " + strings.Join(missing, ", "), Hint: hint}
}

type restCheck struct{}

func (restCheck) Name() string { return "Slurm REST" }
func (restCheck) Run(ctx context.Context) Result {
	keys := []string{"SLURM_API_VERSION", "SLURM_USER"}
	missing := []string{}
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}
	if os.Getenv("SLURM_JWT") == "" && os.Getenv("SLURM_JWT_FILE") == "" {
		missing = append(missing, "SLURM_JWT or SLURM_JWT_FILE")
	}
	if os.Getenv("SLURM_REST_URL") == "" && os.Getenv("SLURM_SOCKET") == "" {
		missing = append(missing, "SLURM_REST_URL or SLURM_SOCKET")
	}
	if len(missing) > 0 {
		if len(missing) == len(keys)+2 {
			return Result{Category: "Slurm", Name: restCheck{}.Name(), Status: SKIP, Detail: "not configured; using cli backend"}
		}
		return Result{Category: "Slurm", Name: restCheck{}.Name(), Status: FAIL, Detail: "missing: " + strings.Join(missing, ", "), Hint: "set the required SLURM_* environment variables"}
	}
	if p := os.Getenv("SLURM_JWT_FILE"); p != "" {
		if _, err := os.ReadFile(p); err != nil {
			return Result{Category: "Slurm", Name: restCheck{}.Name(), Status: FAIL, Detail: "JWT file is not readable", Hint: "set SLURM_JWT_FILE to a readable token file"}
		}
	}
	return Result{Category: "Slurm", Name: restCheck{}.Name(), Status: OK, Detail: "REST configuration is present; token contents are hidden"}
}

type storageCheck struct{ dir string }

func (c storageCheck) Name() string { return "Storage" }
func (c storageCheck) Run(ctx context.Context) Result {
	dir := c.dir
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return Result{Category: "Storage", Name: c.Name(), Status: FAIL, Detail: err.Error(), Hint: "choose a writable database directory"}
	}
	f, err := os.CreateTemp(dir, ".clutch-doctor-*")
	if err != nil {
		return Result{Category: "Storage", Name: c.Name(), Status: FAIL, Detail: err.Error(), Hint: "choose a writable database directory"}
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return Result{Category: "Storage", Name: c.Name(), Status: OK, Detail: filepath.Clean(dir) + " is writable"}
}

func DefaultChecks(dbPath string) []Check {
	return []Check{
		commandCheck{category: "Slurm", name: "Slurm CLI", commands: []string{"sbatch", "squeue", "scontrol", "sacct", "scancel"}},
		restCheck{},
		openFOAMCheck{},
		commandCheck{category: "MPI", name: "MPI", commands: []string{"mpirun", "srun"}},
		storageCheck{dir: filepath.Dir(dbPath)},
	}
}

type openFOAMCheck struct{}

func (openFOAMCheck) Name() string { return "OpenFOAM" }
func (openFOAMCheck) Run(ctx context.Context) Result {
	bashrc := strings.TrimSpace(os.Getenv("FOAM_BASHRC"))
	if bashrc == "" {
		matches, _ := filepath.Glob("/opt/openfoam*/etc/bashrc")
		if len(matches) > 0 {
			sort.Strings(matches)
			bashrc = matches[len(matches)-1]
		}
	}
	if bashrc == "" {
		return Result{Category: "OpenFOAM", Name: "OpenFOAM", Status: FAIL, Detail: "OpenFOAM bashrc was not found", Hint: "set FOAM_BASHRC or pass runtime.foamBashrc in the manifest"}
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", "source \"$1\" && command -v foamDictionary checkMesh decomposePar", "bash", bashrc)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{Category: "OpenFOAM", Name: "OpenFOAM", Status: FAIL, Detail: strings.TrimSpace(string(out)), Hint: "source a valid OpenFOAM bashrc and install foamDictionary, checkMesh, and decomposePar"}
	}
	return Result{Category: "OpenFOAM", Name: "OpenFOAM", Status: OK, Detail: "bashrc loaded; foamDictionary, checkMesh, and decomposePar available"}
}

func SortResults(results []Result) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Category == results[j].Category {
			return results[i].Name < results[j].Name
		}
		return results[i].Category < results[j].Category
	})
}
