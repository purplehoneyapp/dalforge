package dal

// TelemetryProvider defines the interface for recording DAL metrics.
// Injecting this interface allows for easy mocking during unit tests
// and prevents tight coupling to the global Prometheus state.
type TelemetryProvider interface {
	// General DAL metrics
	IncDALOperation(entity, operation string)

	// Database metrics
	IncDBRequest(entity, operation string)
	IncDBError(entity, operation string)
	ObserveDBLatency(entity, operation string, durationSeconds float64)

	// Cache metrics
	IncCacheWrite(entity string)
	IncCacheDelete(entity string)
	IncCacheHit(entity, operation string)
	IncCacheMiss(entity, operation string)
	IncCacheError(entity, operation string)
	SetCacheSize(entity string, size float64)
	ObserveCacheLatency(entity, operation string, durationSeconds float64)

	// Circuit Breaker metrics
	SetCircuitBreakerState(name string, state float64)

	// PubSub Cache Provider metrics
	IncCachePubSubPublish(server string)
	IncCachePubSubReceive(server string)
	IncCachePubSubError(server string)
}

// NoopTelemetryProvider is a dummy implementation used primarily for unit testing.
// It silently discards all telemetry data, keeping the test environment pure.
type NoopTelemetryProvider struct{}

func (p NoopTelemetryProvider) IncDALOperation(entity, operation string)                           {}
func (p NoopTelemetryProvider) IncDBRequest(entity, operation string)                              {}
func (p NoopTelemetryProvider) IncDBError(entity, operation string)                                {}
func (p NoopTelemetryProvider) ObserveDBLatency(entity, operation string, durationSeconds float64) {}

func (p NoopTelemetryProvider) IncCacheWrite(entity string)              {}
func (p NoopTelemetryProvider) IncCacheDelete(entity string)             {}
func (p NoopTelemetryProvider) IncCacheHit(entity, operation string)     {}
func (p NoopTelemetryProvider) IncCacheMiss(entity, operation string)    {}
func (p NoopTelemetryProvider) IncCacheError(entity, operation string)   {}
func (p NoopTelemetryProvider) SetCacheSize(entity string, size float64) {}
func (p NoopTelemetryProvider) ObserveCacheLatency(entity, operation string, durationSeconds float64) {
}

func (p NoopTelemetryProvider) SetCircuitBreakerState(name string, state float64) {}

func (p NoopTelemetryProvider) IncCachePubSubPublish(server string) {}
func (p NoopTelemetryProvider) IncCachePubSubReceive(server string) {}
func (p NoopTelemetryProvider) IncCachePubSubError(server string)   {}
