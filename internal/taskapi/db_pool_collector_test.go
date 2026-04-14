package taskapi

import (
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/prometheus/client_golang/prometheus"
)

func TestSQLDBStatsCollector_gather(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(NewSQLDBStatsCollector(sqlDB))

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, mf := range mfs {
		if mf.Name != nil {
			names = append(names, mf.GetName())
		}
	}
	body := strings.Join(names, "\n")
	for _, needle := range []string{
		"taskapi_db_pool_open_connections",
		"taskapi_db_pool_in_use_connections",
		"taskapi_db_pool_idle_connections",
		"taskapi_db_pool_max_open_connections",
		"taskapi_db_pool_wait_count_total",
		"taskapi_db_pool_wait_duration_seconds_total",
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("missing metric family %q in:\n%s", needle, body)
		}
	}
}
