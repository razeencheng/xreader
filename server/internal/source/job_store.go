package source

import "sync"

type JobStatus struct {
	Status    string  `json:"status"`
	Total     int     `json:"total"`
	Succeeded int     `json:"succeeded"`
	Failed    int     `json:"failed"`
	Skipped   int     `json:"skipped"`
	Progress  float64 `json:"progress"`
	Error     string  `json:"error,omitempty"`
}

func (s JobStatus) withProgress() JobStatus {
	if s.Total <= 0 {
		if s.Status == "done" {
			s.Progress = 1
		}
		return s
	}

	completed := s.Succeeded + s.Failed + s.Skipped
	if completed < 0 {
		completed = 0
	}
	if completed > s.Total {
		completed = s.Total
	}
	s.Progress = float64(completed) / float64(s.Total)
	return s
}

type JobStore interface {
	Get(jobID string) (JobStatus, bool)
	Set(jobID string, status JobStatus)
}

type MemoryJobStore struct {
	jobs sync.Map
}

func NewMemoryJobStore() *MemoryJobStore {
	return &MemoryJobStore{}
}

func (s *MemoryJobStore) Get(jobID string) (JobStatus, bool) {
	v, ok := s.jobs.Load(jobID)
	if !ok {
		return JobStatus{}, false
	}
	return v.(JobStatus), true
}

func (s *MemoryJobStore) Set(jobID string, status JobStatus) {
	s.jobs.Store(jobID, status.withProgress())
}
