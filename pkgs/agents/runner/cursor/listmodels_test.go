package cursor

import (
	"strings"
	"testing"
)

func TestParseListModelsOutput_sampleCLI(t *testing.T) {
	t.Parallel()
	raw := `Available models

auto - Auto
composer-2-fast - Composer 2 Fast (default)
gpt-5.2-codex - Codex 5.2
`
	got := parseListModelsOutput([]byte(raw))
	if len(got) != 3 {
		t.Fatalf("len=%d want 3: %+v", len(got), got)
	}
	if got[0].ID != "auto" || got[0].Label != "Auto" {
		t.Errorf("first: %+v", got[0])
	}
	if got[1].ID != "composer-2-fast" || !strings.Contains(got[1].Label, "Composer 2 Fast") {
		t.Errorf("second: %+v", got[1])
	}
}
