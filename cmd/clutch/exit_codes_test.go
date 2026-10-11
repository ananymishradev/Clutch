package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPipeExitCodes(t *testing.T) {
	tests := []struct {
		name, state string
		want        int
	}{
		{"done", "COMPLETED", exitOK},
		{"failed", "FAILED", exitJob},
		{"cancelled", "CANCELLED", exitJob},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, got := runPipe(t, []string{"run", "--interval", "10ms"}, tt.state)
			if got != tt.want {
				t.Fatalf("state %s exit code = %d, want %d", tt.state, got, tt.want)
			}
		})
	}
}

func TestPTYExitCodes(t *testing.T) {
	tests := []struct {
		name, state string
		want        int
	}{
		{"done", "COMPLETED", exitOK},
		{"failed", "FAILED", exitJob},
		{"cancelled", "CANCELLED", exitJob},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, manifest, db := writeCaseManifest(t)
			s := startPTY(t, []string{"run", "--manifest", manifest, "--db", db, "--backend", "cli", "--interval", "10ms"}, nil)
			s.WaitFor("submitted", 10*time.Second)
			if err := os.WriteFile(filepath.Join(fakeDirFromOutput(t, s), "state"), []byte(tt.state+"\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			got, _ := s.Wait(10 * time.Second)
			if got != tt.want {
				t.Fatalf("state %s exit code = %d, want %d; output=%q", tt.state, got, tt.want, s.Output())
			}
		})
	}
}

func TestPipeUsageAndValidationExitCodes(t *testing.T) {
	_, manifest, _ := writeCaseManifest(t)
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"usage", []string{"not-a-command"}, exitUsage},
		{"validation", []string{"validate", "--manifest", filepath.Join(filepath.Dir(manifest), "missing.yaml")}, exitUsage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, got := runCommandPipes(t, tt.args, nil)
			if got != tt.want {
				t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", got, tt.want, stdout, stderr)
			}
		})
	}
}

func runCommandPipes(t *testing.T, args []string, extra map[string]string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(clutchBinary, args...)
	env := filteredEnv()
	env = append(env, "TERM=dumb", "HOME="+t.TempDir(), "PATH="+fakeBinDir(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	cmd.Env = env
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	}
	t.Fatal(err)
	return "", "", -1
}
