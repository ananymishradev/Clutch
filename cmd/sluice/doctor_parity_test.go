package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func doctorTools(t *testing.T) (bin, bashrc string) {
	t.Helper()
	bin = t.TempDir()
	for _, name := range []string{"mpirun", "srun", "foamDictionary", "checkMesh", "decomposePar"} {
		path := filepath.Join(bin, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	bashrc = filepath.Join(t.TempDir(), "bashrc")
	if err := os.WriteFile(bashrc, []byte("# fake OpenFOAM environment\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	return bin, bashrc
}

func TestPipeDoctorFailExitCode(t *testing.T) {
	_, manifest, db := writeCaseManifest(t)
	bin, bashrc := doctorTools(t)
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "state"), []byte("RUNNING\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), "/etc/bash.bashrc", bashrc))
	if err := os.WriteFile(manifest, data, 0o640); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCommandPipes(t, []string{"doctor", "--db", db, "--manifest", manifest}, map[string]string{
		"JOBREQUEUE":    "0",
		"FAKESLURM_DIR": fake,
		"PATH":          bin + string(os.PathListSeparator) + fakeBinDir(t) + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if code != exitEnvironment {
		t.Fatalf("doctor exit code = %d, want %d; stdout=%q stderr=%q", code, exitEnvironment, stdout, stderr)
	}
	if !strings.Contains(stderr, "requeue disabled") {
		t.Fatalf("doctor failure did not report requeue policy: %q", stderr)
	}
	assertPlainPipe(t, stdout, stderr)
}

func TestPipeDoctorAllPassExitCode(t *testing.T) {
	_, manifest, db := writeCaseManifest(t)
	bin, bashrc := doctorTools(t)
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "state"), []byte("RUNNING\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), "/etc/bash.bashrc", bashrc))
	if err := os.WriteFile(manifest, data, 0o640); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCommandPipes(t, []string{"doctor", "--db", db, "--manifest", manifest}, map[string]string{
		"FAKESLURM_DIR": fake,
		"PATH":          bin + string(os.PathListSeparator) + fakeBinDir(t) + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if code != exitOK {
		t.Fatalf("doctor all-pass exit code = %d; stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertPlainPipe(t, stdout, stderr)
}

func TestPTYDoctorFailExitCode(t *testing.T) {
	_, manifest, db := writeCaseManifest(t)
	bin, bashrc := doctorTools(t)
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), "/etc/bash.bashrc", bashrc))
	if err := os.WriteFile(manifest, data, 0o640); err != nil {
		t.Fatal(err)
	}
	s := startPTY(t, []string{"doctor", "--db", db, "--manifest", manifest}, map[string]string{
		"JOBREQUEUE": "0",
		"PATH":       bin + string(os.PathListSeparator) + fakeBinDir(t) + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	code, _ := s.Wait(10 * time.Second)
	if code != exitEnvironment {
		t.Fatalf("doctor PTY exit code = %d, want %d; output=%q", code, exitEnvironment, s.Output())
	}
}
