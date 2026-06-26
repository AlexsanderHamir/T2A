package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func parseProjectContextPath(r *http.Request) (projectID, itemID string, err error) {
	projectID, err = parseTaskPathID(r.PathValue("id"))
	if err != nil {
		return "", "", err
	}
	itemID, err = parseTaskPathID(r.PathValue("contextId"))
	if err != nil {
		return "", "", err
	}
	return projectID, itemID, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func parseProjectContextEdgePath(r *http.Request) (projectID, edgeID string, err error) {
	projectID, err = parseTaskPathID(r.PathValue("id"))
	if err != nil {
		return "", "", err
	}
	edgeID, err = parseTaskPathID(r.PathValue("edgeId"))
	if err != nil {
		return "", "", err
	}
	return projectID, edgeID, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func parseProjectListParams(q map[string][]string) (limit int, includeArchived bool, err error) {
	limit, err = parseBoundedLimit(q, 50, 100)
	if err != nil {
		return 0, false, err
	}
	includeArchived = strings.EqualFold(strings.TrimSpace(firstQueryValue(q, "include_archived")), "true")
	return limit, includeArchived, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func parseProjectContextListParams(q map[string][]string) (limit int, includeUnpinned bool, err error) {
	limit, err = parseBoundedLimit(q, 50, 100)
	if err != nil {
		return 0, false, err
	}
	includeUnpinned = !strings.EqualFold(strings.TrimSpace(firstQueryValue(q, "pinned_only")), "true")
	return limit, includeUnpinned, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func parseBoundedLimit(q map[string][]string, def, max int) (int, error) {
	raw := strings.TrimSpace(firstQueryValue(q, "limit"))
	if raw == "" {
		return def, nil
	}
	if len(raw) > maxListIntQueryParamBytes {
		return 0, fmt.Errorf("%w: limit value too long", domain.ErrInvalidInput)
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > max {
		return 0, fmt.Errorf("%w: limit must be integer 0..%d", domain.ErrInvalidInput, max)
	}
	if n == 0 {
		return def, nil
	}
	return n, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func firstQueryValue(q map[string][]string, key string) string {
	values := q[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
