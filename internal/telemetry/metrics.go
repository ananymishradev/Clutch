package telemetry

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/ananymishradev/sluice/internal/brand"
)

// Metrics are process-local Prometheus counters. They cover the full job
// lifecycle so dashboards can alert on submission rate, preflight rejects,
// cache efficiency, requeues, and terminal outcomes without parsing logs.
type Metrics struct {
	Submissions       atomic.Uint64
	PreflightFailures atomic.Uint64
	StageCacheHits    atomic.Uint64
	StageCacheMisses  atomic.Uint64
	Requeues          atomic.Uint64
	JobsDone          atomic.Uint64
	JobsFailed        atomic.Uint64
	JobsCancelled     atomic.Uint64
	WatchErrors       atomic.Uint64
}

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		p := brand.MetricPrefix
		lines := []string{
			"# HELP " + p + "submissions_total Slurm submissions accepted.",
			"# TYPE " + p + "submissions_total counter",
			p + "submissions_total " + strconv.FormatUint(m.Submissions.Load(), 10),
			"# HELP " + p + "preflight_failures_total manifests/cases rejected before staging.",
			"# TYPE " + p + "preflight_failures_total counter",
			p + "preflight_failures_total " + strconv.FormatUint(m.PreflightFailures.Load(), 10),
			"# HELP " + p + "stage_cache_hits_total cache reuses.",
			"# TYPE " + p + "stage_cache_hits_total counter",
			p + "stage_cache_hits_total " + strconv.FormatUint(m.StageCacheHits.Load(), 10),
			"# HELP " + p + "stage_cache_misses_total cache populates.",
			"# TYPE " + p + "stage_cache_misses_total counter",
			p + "stage_cache_misses_total " + strconv.FormatUint(m.StageCacheMisses.Load(), 10),
			"# HELP " + p + "requeues_total Slurm requeue-class observations.",
			"# TYPE " + p + "requeues_total counter",
			p + "requeues_total " + strconv.FormatUint(m.Requeues.Load(), 10),
			"# HELP " + p + "jobs_done_total jobs that reached DONE.",
			"# TYPE " + p + "jobs_done_total counter",
			p + "jobs_done_total " + strconv.FormatUint(m.JobsDone.Load(), 10),
			"# HELP " + p + "jobs_failed_total jobs that reached FAILED.",
			"# TYPE " + p + "jobs_failed_total counter",
			p + "jobs_failed_total " + strconv.FormatUint(m.JobsFailed.Load(), 10),
			"# HELP " + p + "jobs_cancelled_total jobs that reached CANCELLED.",
			"# TYPE " + p + "jobs_cancelled_total counter",
			p + "jobs_cancelled_total " + strconv.FormatUint(m.JobsCancelled.Load(), 10),
			"# HELP " + p + "watch_errors_total supervisor polls that errored.",
			"# TYPE " + p + "watch_errors_total counter",
			p + "watch_errors_total " + strconv.FormatUint(m.WatchErrors.Load(), 10),
		}
		_, _ = fmt.Fprintln(w, strings.Join(lines, "\n"))
	})
}
