package model

import (
	"time"

	"gorm.io/datatypes"
)

// AppSettings is the GORM persistence shape for domain.AppSettings.
type AppSettings struct {
	ID                          uint           `gorm:"primaryKey;autoIncrement:false;check:chk_app_settings_singleton,id = 1"`
	AgentPaused                 bool           `gorm:"not null;default:false"`
	Runner                      string         `gorm:"not null;default:'cursor'"`
	CursorBin                   string         `gorm:"not null;default:''"`
	CursorModel                 string         `gorm:"not null;default:''"`
	MaxRunDurationSeconds       int            `gorm:"not null;default:0;check:chk_app_settings_max_run_duration_seconds,max_run_duration_seconds >= 0"`
	StreamIdleStuckSeconds      int            `gorm:"not null;default:60;check:chk_app_settings_stream_idle_stuck_seconds,stream_idle_stuck_seconds >= 0"`
	AgentPickupDelaySeconds     int            `gorm:"not null;default:5;check:chk_app_settings_agent_pickup_delay_seconds,agent_pickup_delay_seconds >= 0"`
	DisplayTimezone             string         `gorm:"not null;default:''"`
	OptimisticMutationsEnabled  bool           `gorm:"not null;default:true"`
	SSEReplayEnabled            bool           `gorm:"not null;default:true"`
	RunnerConfigs               datatypes.JSON `gorm:"column:runner_configs;type:jsonb;not null;default:'{}'"`
	VerifyMaxRetries            int            `gorm:"not null;default:2;check:chk_app_settings_verify_max_retries,verify_max_retries >= 0"`
	VerifyRunnerName            string         `gorm:"not null;default:''"`
	VerifyRunnerModel           string         `gorm:"not null;default:''"`
	VerifyCommandTimeoutSeconds int            `gorm:"not null;default:120;check:chk_app_settings_verify_command_timeout_seconds,verify_command_timeout_seconds > 0"`
	CursorSessionResumeEnabled  bool           `gorm:"not null;default:true"`
	UpdatedAt                   time.Time      `gorm:"not null"`
}

// TableName pins the table name for schema parity with domain.AppSettings.
func (AppSettings) TableName() string { return "app_settings" }
