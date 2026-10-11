package main

import (
	"reflect"
	"testing"
)

func TestRewriteLegacyArgs(t *testing.T) {
	got := rewriteLegacyArgs([]string{"clutch", "run", "-manifest", "case.yaml", "-db", "jobs.db", "-x"})
	want := []string{"clutch", "run", "--manifest", "case.yaml", "--db", "jobs.db", "-x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rewriteLegacyArgs() = %#v, want %#v", got, want)
	}
}

func TestRewriteBareManifest(t *testing.T) {
	got := rewriteLegacyArgs([]string{"clutch", "-manifest", "case.yaml"})
	want := []string{"clutch", "run", "--manifest", "case.yaml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rewriteLegacyArgs() = %#v, want %#v", got, want)
	}
}

func TestRewriteLeavesValuesAndShortFlagsAlone(t *testing.T) {
	got := rewriteLegacyArgs([]string{"clutch", "run", "-m", "a-manifest", "-unknown", "value"})
	want := []string{"clutch", "run", "-m", "a-manifest", "-unknown", "value"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rewriteLegacyArgs() = %#v, want %#v", got, want)
	}
}
