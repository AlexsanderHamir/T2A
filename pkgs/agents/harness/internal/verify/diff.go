package verify

import (
	"os/exec"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt"
)

func gitDiff(dir, rev string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "diff", rev)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	if len(out) > 200*1024 {
		return string(out[:200*1024]) + "\n…(truncated)", nil
	}
	return string(out), nil
}

// DiffSection renders the git diff block for verify and resume prompts.
func DiffSection(workingDir string) string {
	diff, err := gitDiff(workingDir, "HEAD")
	return prompt.FormatVerifyDiffSection(diff, err)
}
