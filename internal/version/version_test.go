package version

import "testing"

func TestString_nonEmpty(t *testing.T) {
	if String() == "" {
		t.Fatal("String returned empty")
	}
}
