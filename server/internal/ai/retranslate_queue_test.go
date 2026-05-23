package ai

import "testing"

func TestRetranslateQueue_DedupesUntilComplete(t *testing.T) {
	q := NewRetranslateQueue(8)

	if !q.Enqueue(1, "zh-CN") {
		t.Fatal("first enqueue should be accepted")
	}
	if q.Enqueue(1, "zh-CN") {
		t.Fatal("duplicate enqueue should be rejected while in-flight")
	}
	if !q.Enqueue(1, "ja-JP") {
		t.Fatal("same article, different language is a distinct job")
	}

	job := <-q.Jobs()
	if job.ArticleID != 1 || job.TargetLang != "zh-CN" {
		t.Fatalf("unexpected job: %+v", job)
	}
	// Still in-flight (not Completed yet) -> still rejected.
	if q.Enqueue(1, "zh-CN") {
		t.Fatal("should stay de-duped until Complete")
	}
	q.Complete(job)
	if !q.Enqueue(1, "zh-CN") {
		t.Fatal("after Complete the job may be re-enqueued")
	}
}

func TestRetranslateQueue_BoundedDropsWhenFull(t *testing.T) {
	q := NewRetranslateQueue(1)

	if !q.Enqueue(1, "zh-CN") {
		t.Fatal("first job should fit the buffer")
	}
	// Buffer (cap 1) holds job 1, unconsumed. A distinct job has nowhere to go.
	if q.Enqueue(2, "zh-CN") {
		t.Fatal("enqueue should fail when the bounded channel is full")
	}
	// Failed enqueue must not leave a stale reservation: after draining,
	// job 2 can be enqueued.
	<-q.Jobs()
	if !q.Enqueue(2, "zh-CN") {
		t.Fatal("job 2 should be enqueueable once space frees up")
	}
}

func TestRetranslateQueue_NonPositiveBufferIsUsable(t *testing.T) {
	q := NewRetranslateQueue(0)
	if !q.Enqueue(1, "zh-CN") {
		t.Fatal("queue with coerced buffer should still accept one job")
	}
}
