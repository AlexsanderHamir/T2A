package gitwork_test

import (
	"runtime"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
)

func TestPathKey_matchesSlashAndCaseVariants(t *testing.T) {
	t.Parallel()
	if gitwork.PathKey(`C:\repo\main`) != gitwork.PathKey(`C:/repo/main`) {
		t.Fatal("slash variants should match")
	}
	if gitwork.PathKey(`/repo/main`) != gitwork.PathKey(`/repo/main/`) {
		t.Fatal("trailing slash should be ignored")
	}
	if runtime.GOOS == "windows" {
		if gitwork.PathKey(`C:\Repo\Main`) != gitwork.PathKey(`c:/repo/main`) {
			t.Fatal("case variants should match on Windows")
		}
	}
}
