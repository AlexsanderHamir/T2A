package main

import (
	"testing"
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
