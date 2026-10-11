//go:build linux || darwin

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ananymishradev/foam-clutch/internal/store"
	"github.com/creack/pty"
)

var clutchBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "foam-clutch-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer os.RemoveAll(dir)
	clutchBinary = filepath.Join(dir, "clutch")
	cmd := exec.Command("go", "build", "-o", clutchBinary, ".")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(2)
	}
	os.Exit(m.Run())
}

type ptySession struct {
	t    *testing.T
	cmd  *exec.Cmd
	m    *os.File
	mu   sync.Mutex
	buf  bytes.Buffer
	done chan error
}

func startPTY(t *testing.T, args []string, extraEnv map[string]string) *ptySession {
	t.Helper()
	fakeDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fakeslurm"))
	if err != nil {
		t.Fatal(err)
	}
	fakeState := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeState, "state"), []byte("RUNNING\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(clutchBinary, args...)
	env := filteredEnv()
	env = append(env,
		"TERM=xterm-256color",
		"FAKESLURM_DIR="+fakeState,
		"HOME="+t.TempDir(),
		"PATH="+fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	m, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 100})
	if err != nil {
		t.Skipf("opening a pty is unavailable: %v", err)
	}
	s := &ptySession{t: t, cmd: cmd, m: m, done: make(chan error, 1)}
	go func() {
		var tmp [4096]byte
		for {
			n, err := m.Read(tmp[:])
			if n > 0 {
				s.mu.Lock()
				s.buf.Write(tmp[:n])
				s.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { s.done <- cmd.Wait() }()
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}
		_ = m.Close()
		select {
		case <-s.done:
		case <-time.After(time.Second):
		}
	})
	return s
}

func filteredEnv() []string {
	var out []string
	for _, item := range os.Environ() {
		key := item
		if i := strings.IndexByte(item, '='); i >= 0 {
			key = item[:i]
		}
		if key == "NO_COLOR" || strings.HasPrefix(key, "FOAM_") || key == "SLURM_REST_URL" || key == "SLURM_SOCKET" || key == "SLURM_API_VERSION" || key == "SLURM_USER" || key == "SLURM_JWT" || key == "SLURM_JWT_FILE" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *ptySession) plain() string {
	s.mu.Lock()
	raw := s.buf.String()
	s.mu.Unlock()
	return stripANSI(raw)
}

func (s *ptySession) Output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func (s *ptySession) WaitFor(substr string, timeout time.Duration) {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(s.plain(), substr) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("timed out waiting for %q; raw output=%q", substr, s.Output())
}

func (s *ptySession) Send(data []byte) {
	s.t.Helper()
	if _, err := s.m.Write(data); err != nil {
		s.t.Fatal(err)
	}
}

func (s *ptySession) Resize(rows, cols uint16) {
	s.t.Helper()
	if err := pty.Setsize(s.m, &pty.Winsize{Rows: rows, Cols: cols}); err != nil {
		s.t.Fatal(err)
	}
}

func (s *ptySession) Wait(timeout time.Duration) (int, error) {
	select {
	case err := <-s.done:
		if err == nil {
			return 0, nil
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), err
		}
		return -1, err
	case <-time.After(timeout):
		s.t.Fatalf("timed out waiting for process; raw output=%q", s.Output())
		return -1, context.DeadlineExceeded
	}
}

func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\a]*(?:\a|\x1b\\))`)
	return re.ReplaceAllString(s, "")
}

func writeCaseManifest(t *testing.T) (root, manifest, db string) {
	t.Helper()
	root = t.TempDir()
	caseDir := filepath.Join(root, "case")
	for _, dir := range []string{"system", "constant", "0"} {
		if err := os.MkdirAll(filepath.Join(caseDir, dir), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		"system/controlDict":           "application simpleFoam;\nrunTimeModifiable true;\n",
		"system/blockMeshDict":         "blocks ();\n",
		"0/p":                          "internalField uniform 0;\n",
		"constant/transportProperties": "nu 1e-05;\n",
	} {
		if err := os.WriteFile(filepath.Join(caseDir, path), []byte(body), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	manifest = filepath.Join(root, "m.yaml")
	content := fmt.Sprintf(`apiVersion: foam-clutch/v1alpha1
name: pty-case
case:
  path: %s
resources:
  nodes: 1
  tasksPerNode: 1
  timeLimitMinutes: 60
solver:
  name: simpleFoam
  checkpointSeconds: 600
mesh:
  strategy: blockMesh
runtime:
  foamBashrc: /etc/bash.bashrc
storage:
  cacheDir: %s
  runDir: %s
`, caseDir, filepath.Join(root, "cache"), filepath.Join(root, "runs"))
	if err := os.WriteFile(manifest, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
	db = filepath.Join(root, "jobs.db")
	return root, manifest, db
}

func localID(t *testing.T, text string) string {
	t.Helper()
	re := regexp.MustCompile(`local ([a-f0-9-]+)`)
	m := re.FindStringSubmatch(text)
	if len(m) != 2 {
		t.Fatalf("local ID not found in %q", text)
	}
	return m[1]
}

func localIDFromDB(t *testing.T, db string) string {
	t.Helper()
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	jobs, err := st.List(context.Background())
	if err != nil || len(jobs) == 0 {
		t.Fatalf("job not found in database: %v", err)
	}
	return jobs[0].ID
}

func fakeDirFromOutput(t *testing.T, s *ptySession) string {
	t.Helper()
	// The harness keeps this in the environment; recover it from the child.
	for _, e := range s.cmd.Env {
		if strings.HasPrefix(e, "FAKESLURM_DIR=") {
			return strings.TrimPrefix(e, "FAKESLURM_DIR=")
		}
	}
	t.Fatal("FAKESLURM_DIR missing")
	return ""
}

func fakeBinDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fakeslurm"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPTY_RunHappyPath(t *testing.T) {
	t.Parallel()
	_, manifest, db := writeCaseManifest(t)
	s := startPTY(t, []string{"run", "--manifest", manifest, "--db", db, "--backend", "cli", "--interval", "50ms"}, nil)
	s.WaitFor("manifest", 15*time.Second)
	s.WaitFor("run dir", 15*time.Second)
	s.WaitFor("submitted", 15*time.Second)
	if err := os.WriteFile(filepath.Join(fakeDirFromOutput(t, s), "state"), []byte("COMPLETED\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	s.WaitFor("state:", 15*time.Second)
	code, _ := s.Wait(15 * time.Second)
	if code != 0 {
		t.Fatalf("exit code %d; output=%q", code, s.Output())
	}
	out := s.plain()
	if !strings.Contains(out, "Slurm ID: 48213") || !strings.Contains(out, "log.solver") || !strings.Contains(out, "slurm-48213.out") || !strings.Contains(out, "slurm-48213.err") {
		t.Fatalf("summary missing expected fields: %s", out)
	}
}

func TestPTY_DetachWithCtrlCAndQ(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"\x03", "q"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			_, manifest, db := writeCaseManifest(t)
			s := startPTY(t, []string{"run", "--manifest", manifest, "--db", db, "--backend", "cli", "--interval", "50ms"}, nil)
			s.WaitFor("RUNNING", 15*time.Second)
			id := localIDFromDB(t, db)
			s.Send([]byte(key))
			code, _ := s.Wait(5 * time.Second)
			if code != 0 {
				t.Fatalf("exit code %d; output=%q", code, s.Output())
			}
			out := s.plain()
			if !strings.Contains(out, "clutch status --id "+id+" --refresh") || !strings.Contains(out, "clutch cancel --id "+id) {
				t.Fatalf("detach notice missing: %s", out)
			}
			if b, err := os.ReadFile(filepath.Join(fakeDirFromOutput(t, s), "scancel.log")); err == nil && len(bytes.TrimSpace(b)) != 0 {
				t.Fatalf("scancel was called: %q", b)
			}
			st, err := store.Open(db)
			if err != nil {
				t.Fatal(err)
			}
			job, err := st.Get(context.Background(), id)
			st.Close()
			if err != nil || (job.State != "QUEUED" && job.State != "RUNNING") {
				t.Fatalf("job was not left active: %#v err=%v", job, err)
			}
			status := exec.Command(clutchBinary, "status", "--db", db, "--id", id, "--refresh", "--backend", "cli")
			status.Env = append(filteredEnv(),
				"TERM=dumb",
				"FAKESLURM_DIR="+fakeDirFromOutput(t, s),
				"HOME="+t.TempDir(),
				"PATH="+fakeBinDir(t)+string(os.PathListSeparator)+os.Getenv("PATH"),
			)
			statusOut, err := status.CombinedOutput()
			if err != nil || !strings.Contains(stripANSI(string(statusOut)), "RUNNING") {
				t.Fatalf("non-TTY status did not report active job: err=%v output=%q", err, statusOut)
			}
		})
	}
}

func TestPTY_SIGTERMDetaches(t *testing.T) {
	t.Parallel()
	_, manifest, db := writeCaseManifest(t)
	s := startPTY(t, []string{"run", "--manifest", manifest, "--db", db, "--backend", "cli", "--interval", "50ms"}, nil)
	s.WaitFor("RUNNING", 15*time.Second)
	id := localIDFromDB(t, db)
	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	code, _ := s.Wait(5 * time.Second)
	if code != 0 || !strings.Contains(s.plain(), "clutch status --id "+id+" --refresh") {
		t.Fatalf("SIGTERM did not detach cleanly: code=%d output=%q", code, s.Output())
	}
}

func TestPTY_CtrlCBeforeSubmit(t *testing.T) {
	t.Parallel()
	_, manifest, db := writeCaseManifest(t)
	s := startPTY(t, []string{"run", "--manifest", manifest, "--db", db, "--backend", "cli"}, map[string]string{"FAKESLURM_BLOCK_SBATCH": "1"})
	s.WaitFor("run dir", 15*time.Second)
	s.Send([]byte("\x03"))
	code, _ := s.Wait(5 * time.Second)
	if code != 130 {
		t.Fatalf("expected 130, got %d; output=%q", code, s.Output())
	}
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := st.List(context.Background())
	st.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.State == "QUEUED" || row.State == "RUNNING" {
			t.Fatalf("pre-submit interrupt left submitted job: %#v", row)
		}
	}
}

func TestPTY_TerminalRestoredAndNoAltScreen(t *testing.T) {
	t.Parallel()
	_, manifest, db := writeCaseManifest(t)
	s := startPTY(t, []string{"run", "--manifest", manifest, "--db", db, "--backend", "cli", "--interval", "50ms"}, nil)
	s.WaitFor("RUNNING", 15*time.Second)
	s.Send([]byte("q"))
	s.Wait(5 * time.Second)
	raw := s.Output()
	if strings.Contains(raw, "\x1b[?1049h") {
		t.Fatal("live view entered the alternate screen")
	}
	lastHide, lastShow := strings.LastIndex(raw, "\x1b[?25l"), strings.LastIndex(raw, "\x1b[?25h")
	if lastHide >= 0 && lastShow < lastHide {
		t.Fatal("cursor was not restored")
	}
	if strings.HasSuffix(raw, "\x1b") {
		t.Fatal("output ends mid escape sequence")
	}
}

func TestPTY_NoColorFlagAndEnvironment(t *testing.T) {
	t.Parallel()
	for _, env := range []map[string]string{{}, {"NO_COLOR": "1"}} {
		t.Run(fmt.Sprint(env), func(t *testing.T) {
			t.Parallel()
			_, manifest, db := writeCaseManifest(t)
			args := []string{"run", "--manifest", manifest, "--db", db, "--backend", "cli", "--interval", "50ms"}
			if len(env) == 0 {
				args = append(args, "--no-color")
			}
			args = append(args, "--no-watch")
			s := startPTY(t, args, env)
			s.WaitFor("submitted", 15*time.Second)
			s.Wait(5 * time.Second)
			if regexp.MustCompile(`\x1b\[[0-9;]*m`).MatchString(s.Output()) {
				t.Fatal("color SGR sequence found")
			}
		})
	}
}

func TestPTY_Resize(t *testing.T) {
	t.Parallel()
	_, manifest, db := writeCaseManifest(t)
	s := startPTY(t, []string{"run", "--manifest", manifest, "--db", db, "--backend", "cli", "--interval", "50ms"}, nil)
	s.WaitFor("RUNNING", 15*time.Second)
	s.Resize(30, 50)
	s.WaitFor("RUNNING", 5*time.Second)
	s.Resize(30, 120)
	s.WaitFor("phase:", 5*time.Second)
	s.Send([]byte("q"))
	if code, _ := s.Wait(5 * time.Second); code != 0 {
		t.Fatalf("resize run exited %d; output=%q", code, s.Output())
	}
}

func TestPTY_LogInjection(t *testing.T) {
	t.Parallel()
	_, manifest, db := writeCaseManifest(t)
	s := startPTY(t, []string{"run", "--manifest", manifest, "--db", db, "--backend", "cli", "--interval", "50ms", "--tail", "--tail-interval", "50ms"}, nil)
	s.WaitFor("RUNNING", 15*time.Second)
	id := localIDFromDB(t, db)
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	job, err := st.Get(context.Background(), id)
	st.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(job.RunPath, "log.solver"), []byte("\x1b[31mboom\x1b[0m\n\x1b]0;pwned\x07\nbad\roverwrite\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	s.WaitFor("boom", 5*time.Second)
	s.Send([]byte("q"))
	s.Wait(5 * time.Second)
	raw := s.Output()
	if strings.Contains(raw, "\x1b]0;pwned\x07") || strings.Contains(raw, "\x1b[31mboom") {
		t.Fatalf("raw terminal controls from log reached the PTY: %q", raw)
	}
}

func TestPTY_DependencyIsTestOnly(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("PTY tests are only built on Unix")
	}
	cmd := exec.Command("go", "list", "-deps", ".")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("github.com/creack/pty")) {
		t.Fatal("creack/pty entered the production dependency graph")
	}
}
