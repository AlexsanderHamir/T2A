package calltrace

import (
	"context"
	"testing"
)

func TestPush_Path_chain(t *testing.T) {
	ctx := context.Background()
	ctx = Push(ctx, "tasks.create")
	ctx = Push(ctx, "decodeJSON")
	if got := Path(ctx); got != "tasks.create > decodeJSON" {
		t.Fatalf("Path: %q", got)
	}
}

func TestPath_emptyWithoutPush(t *testing.T) {
	if Path(context.Background()) != "" {
		t.Fatal("expected empty")
	}
}

func TestWithRequestRoot_nil(t *testing.T) {
	if WithRequestRoot(nil, "x") != nil {
		t.Fatal("expected nil")
	}
}
