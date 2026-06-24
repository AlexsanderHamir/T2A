package verify_test

import (
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/harnesstest"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/metricsfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func newHarness(t *testing.T) *harnesstest.Env {
	t.Helper()
	return harnesstest.NewEnv(t, harnesstest.WithPollTimeout(10*time.Second))
}

// recordingMetrics wraps metricsfake for verify-phase tests.
type recordingMetrics struct {
	*metricsfake.RecordingMetrics
}

type recordedVerdict struct {
	Kind   domain.VerifierKind
	Passed bool
}

func newRecordingMetrics() *recordingMetrics {
	return &recordingMetrics{RecordingMetrics: metricsfake.New()}
}

func (m *recordingMetrics) verdictSnapshot() []recordedVerdict {
	out := make([]recordedVerdict, 0, len(m.SnapshotVerdicts()))
	for _, v := range m.SnapshotVerdicts() {
		out = append(out, recordedVerdict{Kind: v.Kind, Passed: v.Passed})
	}
	return out
}

func (m *recordingMetrics) verifyDurationSnapshot() []time.Duration {
	return m.SnapshotVerifyDurations()
}

func (m *recordingMetrics) verifyRetriesSnapshot() []int {
	return m.SnapshotVerifyRetries()
}
