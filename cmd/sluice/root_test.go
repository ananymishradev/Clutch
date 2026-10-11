package main

import (
	"reflect"
	"testing"
)

func TestRewriteLegacyArgs(t *testing.T) {
	got := rewriteLegacyArgs([]string{"sluice", "run", "-manifest", "case.yaml", "-db", "jobs.db", "-x"})
	want := []string{"sluice", "run", "--manifest", "case.yaml", "--db", "jobs.db", "-x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rewriteLegacyArgs() = %#v, want %#v", got, want)
	}
}

func TestRewriteBareManifest(t *testing.T) {
	got := rewriteLegacyArgs([]string{"sluice", "-manifest", "case.yaml"})
	want := []string{"sluice", "run", "--manifest", "case.yaml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rewriteLegacyArgs() = %#v, want %#v", got, want)
	}
}

func TestRewriteLeavesValuesAndShortFlagsAlone(t *testing.T) {
	got := rewriteLegacyArgs([]string{"sluice", "run", "-m", "a-manifest", "-unknown", "value"})
	want := []string{"sluice", "run", "-m", "a-manifest", "-unknown", "value"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rewriteLegacyArgs() = %#v, want %#v", got, want)
	}
}

func TestRewriteBooleanValues(t *testing.T) {
	got := rewriteLegacyArgs([]string{"sluice", "logs", "-follow=false", "-latest", "-refresh=false"})
	want := []string{"sluice", "logs", "--follow=false", "--latest", "--refresh=false"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rewriteLegacyArgs() = %#v, want %#v", got, want)
	}
}
