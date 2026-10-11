package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ananymishradev/foam-clutch/internal/testutil"
)

func runPipe(t *testing.T, args []string, fakeState string) (stdout, stderr string, code int) {
	t.Helper()
	_, manifest, db := writeCaseManifest(t)
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "state"), []byte(fakeState+"\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	fakeBin := fakeBinDir(t)
	commandArgs := append([]string(nil), args...)
	if len(commandArgs) > 0 && commandArgs[0] == "validate" {
		commandArgs = append(commandArgs, "--manifest", manifest)
	} else {
		commandArgs = append(commandArgs, "--manifest", manifest, "--db", db, "--backend", "cli")
	}
	cmd := exec.Command(clutchBinary, commandArgs...)
	env := filteredEnv()
	env = append(env,
		"TERM=dumb",
		"FAKESLURM_DIR="+fake,
		"HOME="+t.TempDir(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	cmd.Env = env
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if err == nil {
		code = 0
	} else if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else {
		t.Fatal(err)
	}
	return out.String(), errOut.String(), code
}

func golden(t *testing.T, name, stream string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", name+"."+stream))
	if err != nil {
		t.Fatal(err)
	}
	return testutil.Normalize(string(data))
}

func assertPlainPipe(t *testing.T, stdout, stderr string) {
	t.Helper()
	all := stdout + stderr
	if strings.Contains(all, "\x1b[") || strings.Contains(all, "\r") {
		t.Fatalf("piped output contains terminal control bytes: %q", all)
	}
	if strings.Contains(all, "OpenFOAM × Slurm") {
		t.Fatal("piped output contains the interactive banner")
	}
	for _, frame := range []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"} {
		if strings.Contains(all, frame) {
			t.Fatalf("piped output contains spinner frame %q", frame)
		}
	}
}

func TestPipe_RunNoWatchMatchesBaseline(t *testing.T) {
	stdout, stderr, code := runPipe(t, []string{"run", "--no-watch"}, "COMPLETED")
	if code != 0 {
		t.Fatalf("run exited %d: stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertPlainPipe(t, stdout, stderr)
	if got, want := testutil.Normalize(stdout), golden(t, "run-done", "stdout"); got != want {
		t.Fatalf("stdout differs from baseline:\n got:\n%s\nwant:\n%s", got, want)
	}
	if got, want := testutil.Normalize(stderr), golden(t, "run-done", "stderr"); got != want {
		t.Fatalf("stderr differs from baseline:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPipe_RunPreflightFailureReportsFindings(t *testing.T) {
	_, manifest, db := writeCaseManifest(t)
	if err := os.WriteFile(filepath.Join(filepath.Dir(manifest), "case", "system", "controlDict"), []byte("#codeStream\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "state"), []byte("COMPLETED\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(clutchBinary, "run", "--manifest", manifest, "--db", db, "--backend", "cli")
	cmd.Env = append(filteredEnv(), "TERM=dumb", "FAKESLURM_DIR="+fake,
		"HOME="+t.TempDir(), "PATH="+fakeBinDir(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	if code != exitUsage {
		t.Fatalf("preflight failure exit code = %d, want %d; stderr=%q", code, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "preflight failed") || !strings.Contains(stderr.String(), "finding(s)") {
		t.Fatalf("preflight findings were not reported: %q", stderr.String())
	}
}

func TestPipe_RunTerminalScenariosMatchBaseline(t *testing.T) {
	for _, tt := range []struct {
		name, state string
		code        int
	}{
		{"failed", "FAILED", exitJob},
		{"cancelled", "CANCELLED", exitJob},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := runPipe(t, []string{"run", "--interval", "10ms"}, tt.state)
			if code != tt.code {
				t.Fatalf("exit code = %d, want %d", code, tt.code)
			}
			assertPlainPipe(t, stdout, stderr)
			if got, want := testutil.Normalize(stdout), golden(t, "run-"+tt.name, "stdout"); got != want {
				t.Fatalf("stdout differs from baseline:\n got:\n%s\nwant:\n%s", got, want)
			}
			if got, want := testutil.Normalize(stderr), golden(t, "run-"+tt.name, "stderr"); got != want {
				const marker = "\nerror: command failed\n"
				if !strings.Contains(got, marker) || strings.TrimSuffix(got[:strings.Index(got, marker)], "\n") != strings.TrimSuffix(want, "\n") {
					t.Fatalf("stderr differs from baseline:\n got:\n%s\nwant:\n%s", got, want)
				}
			}
		})
	}
}

func TestPipe_ValidatePreflightGoldens(t *testing.T) {
	_, manifest, _ := writeCaseManifest(t)
	if err := os.WriteFile(filepath.Join(filepath.Dir(manifest), "case", "system", "controlDict"), []byte("#codeStream\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCommandPipes(t, []string{"validate", "--manifest", manifest}, nil)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	assertPlainPipe(t, stdout, stderr)
	if got, want := testutil.Normalize(stdout), golden(t, "validate-preflight-failure", "stdout"); got != want {
		t.Fatalf("stdout differs from baseline:\n got:\n%s\nwant:\n%s", got, want)
	}
	if got, want := testutil.Normalize(stderr), golden(t, "validate-preflight-failure", "stderr"); got != want {
		t.Fatalf("stderr differs from baseline:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPipe_ValidateJSONOnlyOnStdout(t *testing.T) {
	stdout, stderr, code := runPipe(t, []string{"validate", "--output", "json"}, "COMPLETED")
	if code != 0 {
		t.Fatalf("validate exited %d: stdout=%q stderr=%q", code, stdout, stderr)
	}
	var value any
	if err := json.Unmarshal([]byte(stdout), &value); err != nil {
		t.Fatalf("stdout is not one JSON document: %v; output=%q", err, stdout)
	}
	if strings.Contains(stdout, "\x1b") || strings.Contains(stdout, "\n") && strings.TrimSpace(stdout) == "" {
		t.Fatalf("stdout contains unexpected decoration: %q", stdout)
	}
	assertPlainPipe(t, stdout, stderr)
}

func TestPipe_LogsPathsOnly(t *testing.T) {
	_, manifest, db := writeCaseManifest(t)
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "state"), []byte("COMPLETED\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	env := filteredEnv()
	env = append(env, "FAKESLURM_DIR="+fake, "HOME="+t.TempDir(),
		"PATH="+fakeBinDir(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd := exec.Command(clutchBinary, "logs", "--manifest", manifest, "--db", db, "--latest", "--follow=false")
	cmd.Env, cmd.Stdin = env, strings.NewReader("")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	seed := exec.Command(clutchBinary, "run", "--manifest", manifest, "--db", db, "--backend", "cli", "--no-watch")
	seed.Env = env
	if err := seed.Run(); err != nil {
		t.Fatalf("seed run failed: %v", err)
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("logs failed: %v; stderr=%q", err, errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected three log paths, got %q", out.String())
	}
	for _, line := range lines {
		if !filepath.IsAbs(line) || strings.ContainsAny(line, "\x1b\r") {
			t.Fatalf("invalid log path output %q", line)
		}
	}
	if errOut.Len() != 0 {
		t.Fatalf("logs wrote decoration to stderr: %q", errOut.String())
	}
}

func TestPipe_DryRunKeepsPlainData(t *testing.T) {
	stdout, stderr, code := runPipe(t, []string{"run", "--dry-run"}, "COMPLETED")
	if code != 0 {
		t.Fatalf("dry-run exited %d", code)
	}
	assertPlainPipe(t, stdout, stderr)
}

func TestPipe_StderrDevNullKeepsRunData(t *testing.T) {
	_, manifest, db := writeCaseManifest(t)
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "state"), []byte("COMPLETED\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(clutchBinary, "run", "--manifest", manifest, "--db", db, "--backend", "cli", "--no-watch")
	cmd.Env = append(filteredEnv(),
		"TERM=dumb",
		"FAKESLURM_DIR="+fake,
		"HOME="+t.TempDir(),
		"PATH="+fakeBinDir(t)+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	cmd.Stderr = devNull
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := testutil.Normalize(stdout.String()), golden(t, "run-done", "stdout"); got != want {
		t.Fatalf("stdout differs with stderr redirected:\n got:\n%s\nwant:\n%s", got, want)
	}
}
