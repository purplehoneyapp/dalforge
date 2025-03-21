package dal

// CacheProvider interface defines the required methods.
type CacheProvider interface {
	Connect() error
	InvalidateCache(entityName, cacheKey string) error
	FlushListCache(entityName string) error
	OnCacheInvalidated(entityName string, handler func(string))
	OnCacheFlushList(entityName string, handler func())
	Close()
}
