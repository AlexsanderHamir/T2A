package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestGitErrCode_returnsCodeForGitErr(t *testing.T) {
	tests := []struct {
		code string
	}{
		{GitCodeNotARepository},
		{GitCodePathExists},
		{GitCodeBranchExists},
		{GitCodeBranchCheckedOut},
		{GitCodeHasRunningTask},
		{GitCodeRepositoryNotFound},
		{GitCodeWorktreeNotFound},
		{GitCodeBranchNotFound},
		{GitCodeDuplicate},
		{GitCodeBranchBoundToWorktree},
		{GitCodeProjectRepoMismatch},
		{GitCodeBootstrapMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := NewGitErr(tt.code, "msg for "+tt.code)
			if GitErrCode(err) != tt.code {
				t.Fatalf("GitErrCode=%q want %q", GitErrCode(err), tt.code)
			}
			var ge *GitErr
			if !errors.As(err, &ge) {
				t.Fatal("expected *GitErr")
			}
			if ge.Error() != "msg for "+tt.code {
				t.Fatalf("Error()=%q", ge.Error())
			}
		})
	}
}

func TestGitErrCode_emptyForNonGitErr(t *testing.T) {
	if GitErrCode(nil) != "" {
		t.Fatal("nil err should return empty code")
	}
	if GitErrCode(fmt.Errorf("wrap: %w", ErrNotFound)) != "" {
		t.Fatal("non-GitErr should return empty code")
	}
	if GitErrCode(NewGitErr(GitCodePathExists, "x")) != GitCodePathExists {
		t.Fatal("direct GitErr should return code")
	}
}
