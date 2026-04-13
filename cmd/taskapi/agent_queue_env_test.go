package main

import (
	"testing"
	"time"
)

func TestUserTaskAgentQueueCap(t *testing.T) {
	t.Setenv(userTaskAgentQueueCapEnv, "")
	if userTaskAgentQueueCap() != defaultUserTaskAgentQueueCap {
		t.Fatalf("unset got %d want %d", userTaskAgentQueueCap(), defaultUserTaskAgentQueueCap)
	}
	t.Setenv(userTaskAgentQueueCapEnv, "10")
	if userTaskAgentQueueCap() != 10 {
		t.Fatalf("got %d", userTaskAgentQueueCap())
	}
	t.Setenv(userTaskAgentQueueCapEnv, "0")
	if userTaskAgentQueueCap() != defaultUserTaskAgentQueueCap {
		t.Fatalf("zero should fall back to default, got %d", userTaskAgentQueueCap())
	}
	t.Setenv(userTaskAgentQueueCapEnv, "-1")
	if userTaskAgentQueueCap() != defaultUserTaskAgentQueueCap {
		t.Fatalf("negative should fall back to default, got %d", userTaskAgentQueueCap())
	}
	t.Setenv(userTaskAgentQueueCapEnv, "nope")
	if userTaskAgentQueueCap() != defaultUserTaskAgentQueueCap {
		t.Fatalf("invalid should fall back to default, got %d", userTaskAgentQueueCap())
	}
}

func TestUserTaskAgentReconcileInterval(t *testing.T) {
	t.Setenv(userTaskAgentReconcileIntervalEnv, "")
	if userTaskAgentReconcileInterval() != defaultUserTaskAgentReconcileInterval {
		t.Fatalf("unset got %v want %v", userTaskAgentReconcileInterval(), defaultUserTaskAgentReconcileInterval)
	}
	t.Setenv(userTaskAgentReconcileIntervalEnv, "5m")
	if userTaskAgentReconcileInterval() != 5*time.Minute {
		t.Fatalf("got %v", userTaskAgentReconcileInterval())
	}
	t.Setenv(userTaskAgentReconcileIntervalEnv, "0")
	if userTaskAgentReconcileInterval() != 0 {
		t.Fatal("zero should mean startup-only (no periodic ticker)")
	}
	t.Setenv(userTaskAgentReconcileIntervalEnv, "nope")
	if userTaskAgentReconcileInterval() != defaultUserTaskAgentReconcileInterval {
		t.Fatalf("invalid should fall back to default, got %v", userTaskAgentReconcileInterval())
	}
	t.Setenv(userTaskAgentReconcileIntervalEnv, "-1s")
	if userTaskAgentReconcileInterval() != defaultUserTaskAgentReconcileInterval {
		t.Fatalf("negative duration should fall back to default, got %v", userTaskAgentReconcileInterval())
	}
}
