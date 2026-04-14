package version

import (
	"strings"
	"testing"
)

func TestPrometheusBuildInfoLabels_shape(t *testing.T) {
	v, r, gv := PrometheusBuildInfoLabels()
	if v == "" {
		t.Fatal("empty version")
	}
	if r == "" {
		t.Fatal("empty revision")
	}
	if gv == "" || !strings.HasPrefix(gv, "go") {
		t.Fatalf("go_version: %q", gv)
	}
}
