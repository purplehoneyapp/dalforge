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

// Default Cache Provider that does not use Redis. If DAL entity is not provided with cache provider this one
// will be used.
type NoopCacheProvider struct{}

func (d NoopCacheProvider) Connect() error {
	return nil
}

func (d NoopCacheProvider) Close() {}

func (d NoopCacheProvider) InvalidateCache(entityName, cacheKey string) error {
	return nil
}

func (d NoopCacheProvider) FlushListCache(entityName string) error {
	return nil
}

func (d NoopCacheProvider) OnCacheInvalidated(entityName string, handler func(string)) {}
func (d NoopCacheProvider) OnCacheFlushList(entityName string, handler func())         {}
