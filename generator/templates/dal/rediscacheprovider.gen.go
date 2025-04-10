package dal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// RedisCacheProvider implements CacheProvider using Redis Pub/Sub.
type RedisCacheProvider struct {
	client  *redis.Client
	options *redis.Options
	ctx     context.Context
	cancel  context.CancelFunc

	// handlers maps an entity name to its invalidation handler.
	handlers          map[string]func(string)
	flushListHandlers map[string]func()
	mu                sync.RWMutex

	isClosed bool
}

// NewRedisCacheProvider creates a new instance with the given Redis config.
func NewRedisCacheProvider(addr, password string, db int) *RedisCacheProvider {
	ctx, cancel := context.WithCancel(context.Background())

	provider := &RedisCacheProvider{
		options: &redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
			OnConnect: func(ctx context.Context, cn *redis.Conn) error {
				log.Infof("Connected to redis: [%s]", addr)
				return nil
			},
		},
		ctx:               ctx,
		cancel:            cancel,
		handlers:          make(map[string]func(string)),
		flushListHandlers: make(map[string]func()),
	}

	return provider
}

// Connect establishes a Redis connection with exponential backoff.
func (p *RedisCacheProvider) Connect() error {
	var err error
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second
	maxRetries := 5

	for i := 0; i < maxRetries; i++ {
		p.client = redis.NewClient(p.options)
		_, err = p.client.Ping(p.ctx).Result()
		if err == nil {
			return nil
		}

		log.Errorf("Redis connection failed: %v. Retrying in %v...\n", err, backoff)
		time.Sleep(backoff)
		backoff = time.Duration(float64(backoff) * 1.5)
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	cacheErrorCounter.WithLabelValues(p.options.Addr).Inc()
	log.Warnln("Redis connection permanently failed. Cache invalidation may be unreliable.")
	return fmt.Errorf("failed to connect to Redis after %d retries: %w", maxRetries, err)
}

// InvalidateCache publishes an invalidation message asynchronously (fire and forget).
func (p *RedisCacheProvider) InvalidateCache(entityName, cacheKey string) error {
	if p.isClosed {
		cacheErrorCounter.WithLabelValues(p.options.Addr).Inc()
		return errors.New("cache provider is closed")
	}

	// Execute publish asynchronously.
	go func() {
		channel := fmt.Sprintf("cache_invalidation_%s", entityName)
		message := cacheKey
		maxRetries := 3
		delay := 5 * time.Second

		for i := 0; i < maxRetries; i++ {
			err := p.client.Publish(p.ctx, channel, message).Err()
			if err == nil {
				cachePublishedMessages.WithLabelValues(p.options.Addr).Inc()
				log.Debugf("Published cache invalidation: %s -> %s\n", entityName, cacheKey)
				return
			}

			log.Errorf("Failed to publish cache invalidation: %s -> %s. Retrying in %v...\n", entityName, cacheKey, delay)
			time.Sleep(delay)
		}

		cacheErrorCounter.WithLabelValues(p.options.Addr).Inc()
		log.Errorf("Failed to publish cache invalidation: %s -> %s after %d retries\n", entityName, cacheKey, maxRetries)
	}()
	return nil // Return immediately.
}

// FlushListCache publishes an flush list message asynchronously (fire and forget).
func (p *RedisCacheProvider) FlushListCache(entityName string) error {
	if p.isClosed {
		cacheErrorCounter.WithLabelValues(p.options.Addr).Inc()
		return errors.New("cache provider is closed")
	}

	// Execute publish asynchronously.
	go func() {
		channel := fmt.Sprintf("cache_flush_list_%s", entityName)
		message := entityName
		maxRetries := 3
		delay := 5 * time.Second

		for i := 0; i < maxRetries; i++ {
			err := p.client.Publish(p.ctx, channel, message).Err()
			if err == nil {
				cachePublishedMessages.WithLabelValues(p.options.Addr).Inc()
				log.Debugf("Published cache_flush_list: %s \n", entityName)
				return
			}

			log.Errorf("Failed to publish cache_flush_list: %s. Retrying in %v...\n", entityName, delay)
			time.Sleep(delay)
		}

		cacheErrorCounter.WithLabelValues(p.options.Addr).Inc()
		log.Errorf("Failed to publish cache_flush_list: %s after %d retries\n", entityName, maxRetries)
	}()
	return nil // Return immediately.
}

// OnCacheInvalidated registers a handler for a specific entity.
func (p *RedisCacheProvider) OnCacheInvalidated(entityName string, handler func(string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, exists := p.handlers[entityName]
	if !exists {
		p.handlers[entityName] = handler
		go p.listenForInvalidations(entityName)
		// introduce small wait period for subscription to take effect and we start listening to the channel.
		time.Sleep(2 * time.Second)
	}
}

func (p *RedisCacheProvider) OnCacheFlushList(entityName string, handler func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, exists := p.flushListHandlers[entityName]
	if !exists {
		p.flushListHandlers[entityName] = handler
		go p.listenForFlushList(entityName)
		// introduce small wait period for subscription to take effect and we start listening to the channel.
		time.Sleep(2 * time.Second)
	}
}

// listenForInvalidations subscribes to all entity channels using a pattern.
func (p *RedisCacheProvider) listenForInvalidations(entityName string) {
	channel := fmt.Sprintf("cache_invalidation_%s", entityName)
	pubsub := p.client.Subscribe(p.ctx, channel)
	defer pubsub.Close()

	// We got our pubsub created so lets process incoming messages in their own
	log.Infof("Listening for cache invalidation messages for %s\n", entityName)
	for msg := range pubsub.Channel() {
		log.Debugf("Got cache invalidation message for %s\n", entityName)
		// Extract entity name from the channel name.
		p.mu.RLock()
		handler, exists := p.handlers[entityName]
		p.mu.RUnlock()
		if exists {
			handler(msg.Payload)
			cacheReceivedMessages.WithLabelValues(p.options.Addr).Inc()
		} else {
			log.Warnf("No handler registered for entity: %s\n", entityName)
		}
	}
}

// listenForFlushList subscribes to an entity's flush_list channel
func (p *RedisCacheProvider) listenForFlushList(entityName string) {
	channel := fmt.Sprintf("cache_flush_list_%s", entityName)
	pubsub := p.client.Subscribe(p.ctx, channel)
	defer pubsub.Close()

	// We got our pubsub created so lets process incoming messages in their own
	log.Infof("Listening for cache_flush_list messages for %s\n", entityName)
	for range pubsub.Channel() {
		log.Debugf("Got cache_flush_list message for %s\n", entityName)
		// Extract entity name from the channel name.
		p.mu.RLock()
		handler, exists := p.flushListHandlers[entityName]
		p.mu.RUnlock()
		if exists {
			handler()
			cacheReceivedMessages.WithLabelValues(p.options.Addr).Inc()
		} else {
			log.Warnf("No cache_flush_list handler registered for entity: %s\n", entityName)
		}
	}
}

// Close shuts down the provider.
func (p *RedisCacheProvider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isClosed {
		return
	}
	log.Infoln("Closing RedisCacheProvider...")
	p.cancel()
	if p.client != nil {
		_ = p.client.Close()
	}
	p.isClosed = true
}
