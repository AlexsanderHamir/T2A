package handler

import "testing"

func TestServerVersion_nonEmpty(t *testing.T) {
	if ServerVersion() == "" {
		t.Fatal("ServerVersion returned empty string")
	}
}
