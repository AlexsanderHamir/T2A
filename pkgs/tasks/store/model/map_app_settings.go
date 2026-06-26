package model

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"

// FromDomainAppSettings copies a domain row to its persistence model.
func FromDomainAppSettings(d domain.AppSettings) AppSettings {
	return AppSettings{
		ID:                          d.ID,
		AgentPaused:                 d.AgentPaused,
		Runner:                      d.Runner,
		CursorBin:                   d.CursorBin,
		CursorModel:                 d.CursorModel,
		MaxRunDurationSeconds:       d.MaxRunDurationSeconds,
		StreamIdleStuckSeconds:      d.StreamIdleStuckSeconds,
		AgentPickupDelaySeconds:     d.AgentPickupDelaySeconds,
		DisplayTimezone:             d.DisplayTimezone,
		OptimisticMutationsEnabled:  d.OptimisticMutationsEnabled,
		SSEReplayEnabled:            d.SSEReplayEnabled,
		RunnerConfigs:               d.RunnerConfigs,
		VerifyMaxRetries:            d.VerifyMaxRetries,
		VerifyRunnerName:            d.VerifyRunnerName,
		VerifyRunnerModel:           d.VerifyRunnerModel,
		VerifyCommandTimeoutSeconds: d.VerifyCommandTimeoutSeconds,
		CursorSessionResumeEnabled:  d.CursorSessionResumeEnabled,
		UpdatedAt:                   d.UpdatedAt,
	}
}

// ToDomainAppSettings copies a persistence row to domain.AppSettings.
func ToDomainAppSettings(m AppSettings) domain.AppSettings {
	return domain.AppSettings{
		ID:                          m.ID,
		AgentPaused:                 m.AgentPaused,
		Runner:                      m.Runner,
		CursorBin:                   m.CursorBin,
		CursorModel:                 m.CursorModel,
		MaxRunDurationSeconds:       m.MaxRunDurationSeconds,
		StreamIdleStuckSeconds:      m.StreamIdleStuckSeconds,
		AgentPickupDelaySeconds:     m.AgentPickupDelaySeconds,
		DisplayTimezone:             m.DisplayTimezone,
		OptimisticMutationsEnabled:  m.OptimisticMutationsEnabled,
		SSEReplayEnabled:            m.SSEReplayEnabled,
		RunnerConfigs:               m.RunnerConfigs,
		VerifyMaxRetries:            m.VerifyMaxRetries,
		VerifyRunnerName:            m.VerifyRunnerName,
		VerifyRunnerModel:           m.VerifyRunnerModel,
		VerifyCommandTimeoutSeconds: m.VerifyCommandTimeoutSeconds,
		CursorSessionResumeEnabled:  m.CursorSessionResumeEnabled,
		UpdatedAt:                   m.UpdatedAt,
	}
}
