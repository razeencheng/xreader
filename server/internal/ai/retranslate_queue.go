package ai

import "sync"

// RetranslateJob identifies one (article, target language) unit of on-demand
// AI work. It is comparable so it can key the in-flight de-dup set.
type RetranslateJob struct {
	ArticleID  int64
	TargetLang string
}

// RetranslateQueue is a bounded, de-duplicating hand-off between the article
// list handler (producer) and the sync worker (consumer). Enqueue never blocks:
// it drops the job if an identical one is already pending/in-flight or the
// bounded channel is full. A dropped job is harmless — a later list view of the
// same still-untranslated article will re-enqueue it.
type RetranslateQueue struct {
	jobs     chan RetranslateJob
	mu       sync.Mutex
	inflight map[RetranslateJob]struct{}
}

// NewRetranslateQueue returns a queue whose channel buffers up to `buffer`
// jobs. Non-positive buffers are coerced to 1.
func NewRetranslateQueue(buffer int) *RetranslateQueue {
	if buffer <= 0 {
		buffer = 1
	}
	return &RetranslateQueue{
		jobs:     make(chan RetranslateJob, buffer),
		inflight: make(map[RetranslateJob]struct{}),
	}
}

// Enqueue submits a job unless an identical one is already pending/in-flight or
// the bounded channel is full. Never blocks. Returns true if accepted.
func (q *RetranslateQueue) Enqueue(articleID int64, targetLang string) bool {
	job := RetranslateJob{ArticleID: articleID, TargetLang: targetLang}

	q.mu.Lock()
	if _, dup := q.inflight[job]; dup {
		q.mu.Unlock()
		return false
	}
	q.inflight[job] = struct{}{}
	q.mu.Unlock()

	select {
	case q.jobs <- job:
		return true
	default:
		// Channel full: roll back the reservation so a future list view of
		// the same untranslated article can retry.
		q.mu.Lock()
		delete(q.inflight, job)
		q.mu.Unlock()
		return false
	}
}

// Jobs returns the receive side for the consumer.
func (q *RetranslateQueue) Jobs() <-chan RetranslateJob { return q.jobs }

// Complete releases a job's de-dup reservation so the same article may be
// re-enqueued by a future list view if it is still missing a translation.
func (q *RetranslateQueue) Complete(job RetranslateJob) {
	q.mu.Lock()
	delete(q.inflight, job)
	q.mu.Unlock()
}
