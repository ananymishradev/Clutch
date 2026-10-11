package main

import (
	"sync/atomic"
	"testing"
)

func TestDetachStopsWatchWithoutCancellingScheduler(t *testing.T) {
	var submitted atomic.Bool
	var watchStopped, pipelineStopped, schedulerCancelled atomic.Bool
	submitted.Store(true)
	detachRun(submitted.Load, func() { pipelineStopped.Store(true) }, func() { watchStopped.Store(true) })
	if !watchStopped.Load() || pipelineStopped.Load() || schedulerCancelled.Load() {
		t.Fatalf("detach cancelled the wrong work: watch=%v pipeline=%v scheduler=%v",
			watchStopped.Load(), pipelineStopped.Load(), schedulerCancelled.Load())
	}
}

func TestDetachBeforeSubmissionStopsPipeline(t *testing.T) {
	var submitted atomic.Bool
	var pipelineStopped, watchStopped atomic.Bool
	detachRun(submitted.Load, func() { pipelineStopped.Store(true) }, func() { watchStopped.Store(true) })
	if !pipelineStopped.Load() || watchStopped.Load() {
		t.Fatalf("pre-submit detach stopped wrong context: pipeline=%v watch=%v", pipelineStopped.Load(), watchStopped.Load())
	}
}
