package prompt

import (
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// ProjectContextInput carries project context rows assembled by the harness
// before prompt injection.
type ProjectContextInput struct {
	Project domain.Project
	Items   []domain.ProjectContextItem
	Edges   []domain.ProjectContextEdge
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// BuildProjectContextSection renders the XML-tagged project context block.
func BuildProjectContextSection(in ProjectContextInput) string {
	var b strings.Builder
	b.WriteString("<project_context>\n")
	b.WriteString("Project: ")
	b.WriteString(in.Project.Name)
	b.WriteString("\n")
	if strings.TrimSpace(in.Project.ContextSummary) != "" {
		b.WriteString("Summary: ")
		b.WriteString(strings.TrimSpace(in.Project.ContextSummary))
		b.WriteString("\n")
	}
	for _, item := range in.Items {
		b.WriteString("\n[")
		b.WriteString(string(item.Kind))
		b.WriteString("] ")
		b.WriteString(item.Title)
		b.WriteString("\n")
		b.WriteString(item.Body)
		b.WriteString("\n")
	}
	if len(in.Edges) > 0 {
		itemTitles := make(map[string]string, len(in.Items))
		for _, item := range in.Items {
			itemTitles[item.ID] = item.Title
		}
		b.WriteString("\nRelationships:\n")
		for _, edge := range in.Edges {
			b.WriteString("- ")
			b.WriteString(itemTitles[edge.SourceContextID])
			b.WriteString(" ")
			b.WriteString(string(edge.Relation))
			b.WriteString(" ")
			b.WriteString(itemTitles[edge.TargetContextID])
			b.WriteString(" (strength ")
			b.WriteString(fmt.Sprintf("%d", edge.Strength))
			b.WriteString("/5)")
			if strings.TrimSpace(edge.Note) != "" {
				b.WriteString(": ")
				b.WriteString(strings.TrimSpace(edge.Note))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("</project_context>")
	return b.String()
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// EstimateTokens returns a coarse token estimate for rendered context text.
func EstimateTokens(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return (len([]rune(s)) + 3) / 4
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// WrapWithProjectContext wraps the task prompt in project context when present.
func WrapWithProjectContext(prompt string, projectContext string) string {
	if strings.TrimSpace(projectContext) == "" {
		return prompt
	}
	return projectContext + "\n\n<task_prompt>\n" + prompt + "\n</task_prompt>"
}
