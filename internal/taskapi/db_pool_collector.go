package taskapi

import (
	"database/sql"
	"errors"
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

// sqlDBStatsCollector exposes database/sql pool stats from [sql.DB.Stats] on each scrape.
type sqlDBStatsCollector struct {
	db *sql.DB

	descMaxOpen       *prometheus.Desc
	descOpen          *prometheus.Desc
	descInUse         *prometheus.Desc
	descIdle          *prometheus.Desc
	descWaitCount     *prometheus.Desc
	descWaitDuration  *prometheus.Desc
	descMaxIdleClosed *prometheus.Desc
	descMaxIdleTime   *prometheus.Desc
	descMaxLifetime   *prometheus.Desc
}

// NewSQLDBStatsCollector returns a [prometheus.Collector] for the given [sql.DB] pool.
// It is intended for the default registry used by GET /metrics.
func NewSQLDBStatsCollector(db *sql.DB) prometheus.Collector {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapi.NewSQLDBStatsCollector")
	const ns = "taskapi"
	const sub = "db_pool"
	labels := []string{}
	return &sqlDBStatsCollector{
		db: db,
		descMaxOpen: prometheus.NewDesc(
			prometheus.BuildFQName(ns, sub, "max_open_connections"),
			"Maximum number of open connections to the database (SetMaxOpenConns).",
			labels, nil,
		),
		descOpen: prometheus.NewDesc(
			prometheus.BuildFQName(ns, sub, "open_connections"),
			"Number of established connections both in use and idle.",
			labels, nil,
		),
		descInUse: prometheus.NewDesc(
			prometheus.BuildFQName(ns, sub, "in_use_connections"),
			"Number of connections currently in use.",
			labels, nil,
		),
		descIdle: prometheus.NewDesc(
			prometheus.BuildFQName(ns, sub, "idle_connections"),
			"Number of idle connections.",
			labels, nil,
		),
		descWaitCount: prometheus.NewDesc(
			prometheus.BuildFQName(ns, sub, "wait_count_total"),
			"Total number of connections waited for (cumulative).",
			labels, nil,
		),
		descWaitDuration: prometheus.NewDesc(
			prometheus.BuildFQName(ns, sub, "wait_duration_seconds_total"),
			"Total time blocked waiting for a new connection, in seconds (cumulative).",
			labels, nil,
		),
		descMaxIdleClosed: prometheus.NewDesc(
			prometheus.BuildFQName(ns, sub, "connections_closed_max_idle_total"),
			"Total number of connections closed due to SetMaxIdleConns (cumulative).",
			labels, nil,
		),
		descMaxIdleTime: prometheus.NewDesc(
			prometheus.BuildFQName(ns, sub, "connections_closed_max_idle_time_total"),
			"Total number of connections closed due to SetConnMaxIdleTime (cumulative).",
			labels, nil,
		),
		descMaxLifetime: prometheus.NewDesc(
			prometheus.BuildFQName(ns, sub, "connections_closed_max_lifetime_total"),
			"Total number of connections closed due to SetConnMaxLifetime (cumulative).",
			labels, nil,
		),
	}
}

// Describe implements [prometheus.Collector].
func (c *sqlDBStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.descMaxOpen
	ch <- c.descOpen
	ch <- c.descInUse
	ch <- c.descIdle
	ch <- c.descWaitCount
	ch <- c.descWaitDuration
	ch <- c.descMaxIdleClosed
	ch <- c.descMaxIdleTime
	ch <- c.descMaxLifetime
}

// Collect implements [prometheus.Collector].
func (c *sqlDBStatsCollector) Collect(ch chan<- prometheus.Metric) {
	if c.db == nil {
		return
	}
	s := c.db.Stats()
	ch <- prometheus.MustNewConstMetric(c.descMaxOpen, prometheus.GaugeValue, float64(s.MaxOpenConnections))
	ch <- prometheus.MustNewConstMetric(c.descOpen, prometheus.GaugeValue, float64(s.OpenConnections))
	ch <- prometheus.MustNewConstMetric(c.descInUse, prometheus.GaugeValue, float64(s.InUse))
	ch <- prometheus.MustNewConstMetric(c.descIdle, prometheus.GaugeValue, float64(s.Idle))
	ch <- prometheus.MustNewConstMetric(c.descWaitCount, prometheus.CounterValue, float64(s.WaitCount))
	ch <- prometheus.MustNewConstMetric(c.descWaitDuration, prometheus.CounterValue, s.WaitDuration.Seconds())
	ch <- prometheus.MustNewConstMetric(c.descMaxIdleClosed, prometheus.CounterValue, float64(s.MaxIdleClosed))
	ch <- prometheus.MustNewConstMetric(c.descMaxIdleTime, prometheus.CounterValue, float64(s.MaxIdleTimeClosed))
	ch <- prometheus.MustNewConstMetric(c.descMaxLifetime, prometheus.CounterValue, float64(s.MaxLifetimeClosed))
}

var registerDBPoolOnce sync.Once

// RegisterSQLDBPoolCollector registers a collector on [prometheus.DefaultRegisterer]
// that scrapes pool stats from the [gorm.DB]'s underlying [sql.DB]. Safe to call once
// per process; further calls are no-ops.
func RegisterSQLDBPoolCollector(db *gorm.DB) {
	slog.Debug("trace", "cmd", cmdLog, "operation", "taskapi.RegisterSQLDBPoolCollector")
	if db == nil {
		slog.Warn("skip prometheus db pool collector", "cmd", cmdLog, "operation", "taskapi.RegisterSQLDBPoolCollector", "reason", "nil_gorm_db")
		return
	}
	registerDBPoolOnce.Do(func() {
		sqldb, err := db.DB()
		if err != nil {
			slog.Warn("skip prometheus db pool collector", "cmd", cmdLog, "operation", "taskapi.RegisterSQLDBPoolCollector", "reason", "gorm_db_sql", "err", err)
			return
		}
		col := NewSQLDBStatsCollector(sqldb)
		if err := prometheus.DefaultRegisterer.Register(col); err != nil {
			var dup prometheus.AlreadyRegisteredError
			if errors.As(err, &dup) {
				return
			}
			slog.Warn("prometheus db pool collector register failed", "cmd", cmdLog, "operation", "taskapi.RegisterSQLDBPoolCollector", "err", err)
			return
		}
		slog.Info("prometheus db pool collector registered", "cmd", cmdLog, "operation", "taskapi.RegisterSQLDBPoolCollector")
	})
}
