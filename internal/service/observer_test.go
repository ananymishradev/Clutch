package service

import "testing"

func TestServiceWithoutObserverDoesNotPanic(t *testing.T) {
	s := &Service{}
	s.observe("stage", StageStarted, "detail")
	s.observe("stage", StageDone, "detail")
	s.observe("stage", StageFailed, "detail")
}
