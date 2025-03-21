package dal

// ConfigProvider interface defines the required methods that DAL layer needs
type ConfigProvider interface {
	BlockedReads(entityName string) bool
	BlockedWrites(entityName string) bool
}

// DefaultConfigProvider is an implementation of ConfigProvider that
// always returns true for both BlockedReads and BlockedWrites.
type DefaultConfigProvider struct{}

// BlockedReads always returns true.
func (d DefaultConfigProvider) BlockedReads(entityName string) bool {
	return true
}

// BlockedWrites always returns true.
func (d DefaultConfigProvider) BlockedWrites(entityName string) bool {
	return true
}
