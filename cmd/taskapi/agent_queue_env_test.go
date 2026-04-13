package main

import (
	"testing"
	"time"
)

func TestUserTaskAgentQueueCap(t *testing.T) {
	t.Setenv(userTaskAgentQueueCapEnv, "")
	if userTaskAgentQueueCap() != 0 {
		t.Fatal("unset should be 0")
	}
	t.Setenv(userTaskAgentQueueCapEnv, "10")
	if userTaskAgentQueueCap() != 10 {
		t.Fatalf("got %d", userTaskAgentQueueCap())
	}
	t.Setenv(userTaskAgentQueueCapEnv, "0")
	if userTaskAgentQueueCap() != 0 {
		t.Fatal("zero should disable")
	}
	t.Setenv(userTaskAgentQueueCapEnv, "-1")
	if userTaskAgentQueueCap() != 0 {
		t.Fatal("negative should disable")
	}
	t.Setenv(userTaskAgentQueueCapEnv, "nope")
	if userTaskAgentQueueCap() != 0 {
		t.Fatal("invalid should disable")
	}
}

func TestUserTaskAgentReconcileInterval(t *testing.T) {
	t.Setenv(userTaskAgentReconcileIntervalEnv, "")
	if userTaskAgentReconcileInterval() != 0 {
		t.Fatal("unset should be 0")
	}
	t.Setenv(userTaskAgentReconcileIntervalEnv, "5m")
	if userTaskAgentReconcileInterval() != 5*time.Minute {
		t.Fatalf("got %v", userTaskAgentReconcileInterval())
	}
	t.Setenv(userTaskAgentReconcileIntervalEnv, "0")
	if userTaskAgentReconcileInterval() != 0 {
		t.Fatal("zero should disable periodic")
	}
	t.Setenv(userTaskAgentReconcileIntervalEnv, "nope")
	if userTaskAgentReconcileInterval() != 0 {
		t.Fatal("invalid should be 0")
	}
}
