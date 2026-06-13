package handler

import "strings"

// testCriterionText is the default non-empty done criterion used in handler
// contract tests after POST /tasks began requiring checklist_items.
const testCriterionText = "Test criterion"

// withCreateChecklist injects the required checklist_items field into a POST
// /tasks JSON object. jsonBody must be a single object literal ending with `}`.
// No-op when checklist_items is already present.
func withCreateChecklist(jsonBody string) string {
	jsonBody = strings.TrimSpace(jsonBody)
	if strings.Contains(jsonBody, "checklist_items") {
		return jsonBody
	}
	if !strings.HasSuffix(jsonBody, "}") {
		return jsonBody
	}
	return jsonBody[:len(jsonBody)-1] + `,"checklist_items":[{"text":"` + testCriterionText + `"}]}`
}
