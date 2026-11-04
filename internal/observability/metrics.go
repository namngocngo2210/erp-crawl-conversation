package observability

import "github.com/prometheus/client_golang/prometheus"

var (
	CrawlRunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "crawl_runs_total", Help: "Total crawl runs by status."},
		[]string{"status"},
	)
	MessagesIngested = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "messages_ingested_total", Help: "Total messages ingested."},
	)
	CrawlDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{Name: "crawl_duration_seconds", Help: "Crawl run duration.", Buckets: prometheus.ExponentialBuckets(1, 2, 12)},
	)
)

func init() {
	prometheus.MustRegister(CrawlRunsTotal, MessagesIngested, CrawlDuration)
}
