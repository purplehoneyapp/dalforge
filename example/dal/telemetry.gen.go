package dal

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
)

var (
	HealthRegistry *prometheus.Registry

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

func RegisterTelemetry(registerer prometheus.Registerer) {
	registerer.MustRegister(dalOperationsTotalCounter,
		dbRequestsCounter,
		dbRequestsErrorsCounter,
		dbRequestsLatencyHistogram,
		circuitBreakerStateGauge,
		dalCacheHitsCounter,
		dalCacheMissesCounter,
		dalCacheLatencyHistogram,
		dalCacheWritesCounter,
		dalCacheErrorsCounter,
		dalCacheDeletesCounter,
		dalCacheSizeGauge,
		cachePublishedMessages,
		cacheReceivedMessages,
		cacheErrorCounter)
}

// Resets all vectors in all metrics
func ResetTelemetry() {
	dalOperationsTotalCounter.Reset()
	dbRequestsCounter.Reset()
	dbRequestsErrorsCounter.Reset()
	dbRequestsLatencyHistogram.Reset()
	circuitBreakerStateGauge.Reset()
	dalCacheHitsCounter.Reset()
	dalCacheMissesCounter.Reset()
	dalCacheLatencyHistogram.Reset()
	dalCacheWritesCounter.Reset()
	dalCacheErrorsCounter.Reset()
	dalCacheDeletesCounter.Reset()
	dalCacheSizeGauge.Reset()
	cachePublishedMessages.Reset()
	cacheReceivedMessages.Reset()
	cacheErrorCounter.Reset()
}

// PrometheusTelemetryProvider implements TelemetryProvider using global Prometheus metrics.
// Use this implementation in your production server environment.
type PrometheusTelemetryProvider struct{}

func (p PrometheusTelemetryProvider) IncDALOperation(entity, operation string) {
	dalOperationsTotalCounter.WithLabelValues(entity, operation).Inc()
}

func (p PrometheusTelemetryProvider) IncDBRequest(entity, operation string) {
	dbRequestsCounter.WithLabelValues(entity, operation).Inc()
}

func (p PrometheusTelemetryProvider) IncDBError(entity, operation string) {
	dbRequestsErrorsCounter.WithLabelValues(entity, operation).Inc()
}

func (p PrometheusTelemetryProvider) ObserveDBLatency(entity, operation string, durationSeconds float64) {
	dbRequestsLatencyHistogram.WithLabelValues(entity, operation).Observe(durationSeconds)
}

func (p PrometheusTelemetryProvider) IncCacheWrite(entity string) {
	dalCacheWritesCounter.WithLabelValues(entity).Inc()
}

func (p PrometheusTelemetryProvider) IncCacheDelete(entity string) {
	dalCacheDeletesCounter.WithLabelValues(entity).Inc()
}

func (p PrometheusTelemetryProvider) IncCacheHit(entity, operation string) {
	dalCacheHitsCounter.WithLabelValues(entity, operation).Inc()
}

func (p PrometheusTelemetryProvider) IncCacheMiss(entity, operation string) {
	dalCacheMissesCounter.WithLabelValues(entity, operation).Inc()
}

func (p PrometheusTelemetryProvider) IncCacheError(entity, operation string) {
	dalCacheErrorsCounter.WithLabelValues(entity, operation).Inc()
}

func (p PrometheusTelemetryProvider) SetCacheSize(entity string, size float64) {
	dalCacheSizeGauge.WithLabelValues(entity).Set(size)
}

func (p PrometheusTelemetryProvider) ObserveCacheLatency(entity, operation string, durationSeconds float64) {
	dalCacheLatencyHistogram.WithLabelValues(entity, operation).Observe(durationSeconds)
}

func (p PrometheusTelemetryProvider) SetCircuitBreakerState(name string, state float64) {
	circuitBreakerStateGauge.WithLabelValues(name).Set(state)
}

func (p PrometheusTelemetryProvider) IncCachePubSubPublish(server string) {
	cachePublishedMessages.WithLabelValues(server).Inc()
}

func (p PrometheusTelemetryProvider) IncCachePubSubReceive(server string) {
	cacheReceivedMessages.WithLabelValues(server).Inc()
}

func (p PrometheusTelemetryProvider) IncCachePubSubError(server string) {
	cacheErrorCounter.WithLabelValues(server).Inc()
}
