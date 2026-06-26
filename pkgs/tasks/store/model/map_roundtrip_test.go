package model

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/datatypes"
)

func TestAppSettings_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	cfg := datatypes.JSON(`{"cursor":{"binary_path":"/bin/cursor"}}`)
	orig := domain.AppSettings{
		ID:                          domain.AppSettingsRowID,
		AgentPaused:                 true,
		Runner:                      "cursor",
		CursorBin:                   "/bin/cursor",
		CursorModel:                 "opus",
		MaxRunDurationSeconds:       120,
		StreamIdleStuckSeconds:      45,
		AgentPickupDelaySeconds:     3,
		DisplayTimezone:             "America/Los_Angeles",
		OptimisticMutationsEnabled:  true,
		SSEReplayEnabled:            true,
		RunnerConfigs:               cfg,
		VerifyMaxRetries:            1,
		VerifyRunnerName:            "cursor",
		VerifyRunnerModel:           "gpt",
		VerifyCommandTimeoutSeconds: 90,
		CursorSessionResumeEnabled:  false,
		UpdatedAt:                   now,
	}
	m := FromDomainAppSettings(orig)
	back := ToDomainAppSettings(m)
	if !reflect.DeepEqual(orig, back) {
		t.Fatalf("round-trip mismatch:\norig=%+v\nback=%+v", orig, back)
	}
	m2 := FromDomainAppSettings(back)
	if !appSettingsModelEqual(m, m2) {
		t.Fatalf("model round-trip mismatch")
	}
}

func appSettingsModelEqual(a, b AppSettings) bool {
	return reflect.DeepEqual(a, b)
}

func TestAppSettings_emptyRunnerConfigs(t *testing.T) {
	t.Parallel()
	orig := domain.DefaultAppSettings()
	m := FromDomainAppSettings(orig)
	m2 := FromDomainAppSettings(ToDomainAppSettings(m))
	if !reflect.DeepEqual(m, m2) {
		t.Fatal("empty runner configs round-trip failed")
	}
}

func TestTaskEvent_roundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	resp := "ack"
	respAt := now.Add(time.Minute)
	data := datatypes.JSON(`{"status":"ready"}`)
	thread := datatypes.JSON(`[{"at":"2026-03-01T12:01:00Z","by":"user","body":"hi"}]`)
	orig := domain.TaskEvent{
		TaskID:         "task-1",
		Seq:            2,
		At:             now,
		Type:           domain.EventStatusChanged,
		By:             domain.ActorUser,
		Data:           data,
		UserResponse:   &resp,
		UserResponseAt: &respAt,
		ResponseThread: thread,
	}
	m := FromDomainTaskEvent(orig)
	m2 := FromDomainTaskEvent(ToDomainTaskEvent(m))
	if !taskEventModelEqual(m, m2) {
		t.Fatal("model round-trip mismatch")
	}
	back := ToDomainTaskEvent(m)
	if !jsonEqual(data, back.Data) || !jsonEqual(thread, back.ResponseThread) {
		t.Fatalf("json columns diverged: data=%s thread=%s", back.Data, back.ResponseThread)
	}
	if back.UserResponse == nil || *back.UserResponse != resp {
		t.Fatalf("user response: got %v", back.UserResponse)
	}
}

func TestTaskEvent_nilOptionalFields(t *testing.T) {
	t.Parallel()
	orig := domain.TaskEvent{
		TaskID: "t",
		Seq:    1,
		At:     time.Now().UTC(),
		Type:   domain.EventTaskCreated,
		By:     domain.ActorAgent,
		Data:   datatypes.JSON(`{}`),
	}
	m := FromDomainTaskEvent(orig)
	m2 := FromDomainTaskEvent(ToDomainTaskEvent(m))
	if !taskEventModelEqual(m, m2) {
		t.Fatal("nil optional fields round-trip failed")
	}
}

func taskEventModelEqual(a, b TaskEvent) bool {
	return a.TaskID == b.TaskID &&
		a.Seq == b.Seq &&
		a.At.Equal(b.At) &&
		a.Type == b.Type &&
		a.By == b.By &&
		jsonEqual(a.Data, b.Data) &&
		jsonEqual(a.ResponseThread, b.ResponseThread) &&
		ptrStrEqual(a.UserResponse, b.UserResponse) &&
		ptrTimeEqual(a.UserResponseAt, b.UserResponseAt)
}

func jsonEqual(a, b datatypes.JSON) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var ja, jb any
	if err := json.Unmarshal(a, &ja); err != nil {
		return bytes.Equal(a, b)
	}
	if err := json.Unmarshal(b, &jb); err != nil {
		return bytes.Equal(a, b)
	}
	ma, _ := json.Marshal(ja)
	mb, _ := json.Marshal(jb)
	return bytes.Equal(ma, mb)
}

func ptrStrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func ptrTimeEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
