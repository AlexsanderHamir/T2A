package prompt

import (
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// InjectAutomations prepends the agent-behavior toggle block before the
// operator's initial prompt. Yes-state rows affirm the description; no-state
// rows explicitly prohibit it.
func InjectAutomations(prompt string, items []domain.ResolvedAutomation) string {
	if len(items) == 0 {
		return prompt
	}
	var b strings.Builder
	b.WriteString("## Agent behaviors\n\n")
	for _, it := range items {
		switch it.State {
		case domain.AutomationStateYes:
			b.WriteString(fmt.Sprintf("- [YES] %s: %s\n", it.Title, it.Description))
		case domain.AutomationStateNo:
			b.WriteString(fmt.Sprintf("- [NO] %s: Do NOT %s\n", it.Title, it.Description))
		}
	}
	b.WriteString("\n")
	return b.String() + prompt
}
