package service

type StageStatus string

const (
	StageStarted StageStatus = "started"
	StageDone    StageStatus = "done"
	StageFailed  StageStatus = "failed"
)

type Observer interface {
	Stage(name string, status StageStatus, detail string)
}

func (s *Service) observe(name string, status StageStatus, detail string) {
	if s.Observer != nil {
		s.Observer.Stage(name, status, detail)
	}
}
