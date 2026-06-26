package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

// FromDomainTaskEvent copies a domain row to its persistence model.
func FromDomainTaskEvent(d domain.TaskEvent) TaskEvent {
	return TaskEvent{
		TaskID:         d.TaskID,
		Seq:            d.Seq,
		At:             d.At,
		Type:           d.Type,
		By:             d.By,
		Data:           datatypesFromRaw(d.Data),
		UserResponse:   d.UserResponse,
		UserResponseAt: d.UserResponseAt,
		ResponseThread: datatypesFromRaw(d.ResponseThread),
	}
}

// ToDomainTaskEvent copies a persistence row to domain.TaskEvent.
func ToDomainTaskEvent(m TaskEvent) domain.TaskEvent {
	return domain.TaskEvent{
		TaskID:         m.TaskID,
		Seq:            m.Seq,
		At:             m.At,
		Type:           m.Type,
		By:             m.By,
		Data:           rawJSONObjectFromDatatypes(m.Data),
		UserResponse:   m.UserResponse,
		UserResponseAt: m.UserResponseAt,
		ResponseThread: rawFromDatatypes(m.ResponseThread),
	}
}

// ToDomainTaskEvents maps a slice of persistence rows to domain.TaskEvent.
func ToDomainTaskEvents(rows []TaskEvent) []domain.TaskEvent {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.TaskEvent, len(rows))
	for i := range rows {
		out[i] = ToDomainTaskEvent(rows[i])
	}
	return out
}
