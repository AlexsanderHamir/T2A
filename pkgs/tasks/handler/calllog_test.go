package handler

import (
	"context"
	"testing"
)

func TestPushCall_CallPath_chain(t *testing.T) {
	ctx := context.Background()
	ctx = PushCall(ctx, "tasks.create")
	ctx = PushCall(ctx, "decodeJSON")
	if got := CallPath(ctx); got != "tasks.create > decodeJSON" {
		t.Fatalf("CallPath: %q", got)
	}
}

func TestCallPath_emptyWithoutPush(t *testing.T) {
	if CallPath(context.Background()) != "" {
		t.Fatal("expected empty")
	}
}

func TestWithCallRoot_nilRequest(t *testing.T) {
	if withCallRoot(nil, "x") != nil {
		t.Fatal("expected nil")
	}
}
