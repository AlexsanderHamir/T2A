package handler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

type dependsOnWire []json.RawMessage

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func parseDependsOnWire(raw dependsOnWire) ([]domain.DependencyEdge, error) {
	if raw == nil {
		return nil, nil
	}
	out := make([]domain.DependencyEdge, 0, len(raw))
	for i, item := range raw {
		var asString string
		if err := json.Unmarshal(item, &asString); err == nil {
			id := strings.TrimSpace(asString)
			if id == "" {
				continue
			}
			out = append(out, domain.DependencyEdge{TaskID: id, Satisfies: domain.DependencySatisfiesDone})
			continue
		}
		var obj struct {
			TaskID    string `json:"task_id"`
			Satisfies string `json:"satisfies"`
		}
		if err := json.Unmarshal(item, &obj); err != nil {
			return nil, fmt.Errorf("%w: depends_on[%d] must be task id string or object", domain.ErrInvalidInput, i)
		}
		id := strings.TrimSpace(obj.TaskID)
		if id == "" {
			return nil, fmt.Errorf("%w: depends_on[%d].task_id required", domain.ErrInvalidInput, i)
		}
		satisfies := domain.DependencySatisfies(strings.TrimSpace(obj.Satisfies))
		if satisfies == "" {
			satisfies = domain.DependencySatisfiesDone
		}
		if !domain.ValidDependencySatisfies(satisfies) {
			return nil, fmt.Errorf("%w: invalid depends_on[%d].satisfies", domain.ErrInvalidInput, i)
		}
		out = append(out, domain.DependencyEdge{TaskID: id, Satisfies: domain.NormalizeDependencySatisfies(satisfies)})
	}
	return out, nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (d *dependsOnWire) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*d = nil
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*d = raw
	return nil
}

type dependsOnPatchWire struct {
	set   bool
	value []domain.DependencyEdge
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (d *dependsOnPatchWire) UnmarshalJSON(data []byte) error {
	d.set = true
	if string(data) == "null" {
		d.value = []domain.DependencyEdge{}
		return nil
	}
	var raw dependsOnWire
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	edges, err := parseDependsOnWire(raw)
	if err != nil {
		return err
	}
	d.value = edges
	return nil
}
