package dal

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
)

var (
	dalOperationsTotalCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dal_operations_total",
			Help: "Total number of DAL operation requests",
		},
		[]string{"entity", "operation"},
	)

	dbRequestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_requests_total",
			Help: "Total number of DB requests",
		},
		[]string{"entity", "operation"},
	)

	dbRequestsErrorsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_requests_errors_total",
			Help: "Total number of failed DB requests",
		},
		[]string{"entity", "operation"},
	)

	dbRequestsLatencyHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_requests_latency_seconds",
			Help:    "Latency distribution of DB requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"entity", "operation"},
	)

	dalCacheWritesCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dal_cache_writes_total",
			Help: "Total number of DAL cache writes for this entity",
		},
		[]string{"entity"},
	)
	dalCacheDeletesCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dal_cache_deletes_total",
			Help: "Total number of DAL cache removals (invalidations)",
		},
		[]string{"entity"},
	)
	dalCacheHitsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dal_cache_hits_total",
			Help: "Total number of DAL cache hits",
		},
		[]string{"entity", "operation"},
	)
	dalCacheErrorsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dal_cache_errors_total",
			Help: "Total number of DAL cache errors",
		},
		[]string{"entity", "operation"},
	)
	dalCacheMissesCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dal_cache_misses_total",
			Help: "Total number of DAL cache misses",
		},
		[]string{"entity", "operation"},
	)
	dalCacheSizeGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dal_cache_size",
			Help: "Size of cache in items",
		},
		[]string{"entity"},
	)
	dalCacheLatencyHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dal_cache_latency_seconds",
			Help:    "Latency distribution of DAL cache operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"entity", "operation"},
	)

	circuitBreakerStateGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "Current state of the circuit breaker (0=closed, 1=half-open, 2=open).",
		},
		[]string{"breaker_name"}, // label for identifying which circuit breaker
	)

	// Initialize Prometheus counters for caching.
	cachePublishedMessages = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_invalidation_published_total",
		Help: "Total number of cache invalidation messages published",
	}, []string{"server"})

	cacheReceivedMessages = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_invalidation_received_total",
		Help: "Total number of cache invalidation messages received",
	}, []string{"server"})
	cacheErrorCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_provider_errors_total",
		Help: "Total number of cache provider errors",
	}, []string{"server"})
)

func OnCircuitBreakerStateChange(name string, from gobreaker.State, to gobreaker.State) {
	switch to {
	case gobreaker.StateClosed:
		circuitBreakerStateGauge.WithLabelValues(name).Set(0)
	case gobreaker.StateHalfOpen:
		circuitBreakerStateGauge.WithLabelValues(name).Set(1)
	case gobreaker.StateOpen:
		circuitBreakerStateGauge.WithLabelValues(name).Set(2)
	}
}

func init() {
	prometheus.MustRegister(dalOperationsTotalCounter,
		dbRequestsCounter,
		dbRequestsErrorsCounter,
		dbRequestsLatencyHistogram)
	prometheus.MustRegister(circuitBreakerStateGauge)
	prometheus.MustRegister(dalCacheHitsCounter,
		dalCacheMissesCounter,
		dalCacheLatencyHistogram,
		dalCacheWritesCounter,
		dalCacheErrorsCounter,
		dalCacheDeletesCounter,
		dalCacheSizeGauge)
	prometheus.MustRegister(cachePublishedMessages, cacheReceivedMessages, cacheErrorCounter)
}
