package source

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJobStatusWithProgressReportsCompletedFeedRatio(t *testing.T) {
	status := JobStatus{
		Status:    "running",
		Total:     4,
		Succeeded: 1,
		Failed:    1,
		Skipped:   0,
	}.withProgress()

	require.InDelta(t, 0.5, status.Progress, 0.001)
}

func TestJobStatusWithProgressMarksEmptyDoneJobComplete(t *testing.T) {
	status := JobStatus{
		Status: "done",
		Total:  0,
	}.withProgress()

	require.Equal(t, 1.0, status.Progress)
}
