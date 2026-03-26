package redis

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	dcache "github.com/felixgeelhaar/agent-go/domain/cache"
	"github.com/redis/go-redis/v9"
)

const (
	// invalidationChannel is the default pub/sub channel for distributed invalidation.
	invalidationChannel = "cache:invalidate"
)

// InvalidationMessage represents a message sent over pub/sub for distributed cache invalidation.
type InvalidationMessage struct {
	// Type is the invalidation type: "key", "pattern", or "tag".
	Type string `json:"type"`
	// Value is the key, pattern, or tag to invalidate.
	Value string `json:"value"`
}

// Invalidator provides cache invalidation patterns on top of a cacheBase.
// It supports pattern-based deletion, tag-based grouping, and distributed
// invalidation via Redis pub/sub.
type Invalidator struct {
	base    *cacheBase
	cmd     redis.Cmdable
	channel string

	// subscriber state
	mu     sync.Mutex
	pubsub *redis.PubSub
	stopCh chan struct{}
}

// NewInvalidator creates an Invalidator for the given Cache.
func NewInvalidator(c *Cache) *Invalidator {
	return &Invalidator{
		base:    &c.cacheBase,
		cmd:     c.client,
		channel: c.cacheBase.keyPrefix + invalidationChannel,
	}
}

// NewClusterInvalidator creates an Invalidator for the given ClusterCache.
func NewClusterInvalidator(cc *ClusterCache) *Invalidator {
	return &Invalidator{
		base:    &cc.cacheBase,
		cmd:     cc.client,
		channel: cc.cacheBase.keyPrefix + invalidationChannel,
	}
}

// InvalidatePattern deletes all cache keys matching the given glob pattern.
// The pattern is applied within the cache key namespace. For example, pattern
// "user:*" will match keys like "agent:cache:user:123".
func (inv *Invalidator) InvalidatePattern(ctx context.Context, pattern string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if pattern == "" {
		return 0, dcache.ErrInvalidKey
	}

	fullPattern := inv.base.keyPrefix + "cache:" + pattern
	return inv.deleteByPattern(ctx, fullPattern)
}

// SetWithTags stores a value and associates it with the given tags.
// Tags enable group invalidation: invalidating a tag removes all keys
// associated with that tag.
func (inv *Invalidator) SetWithTags(ctx context.Context, key string, value []byte, opts dcache.SetOptions, tags ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if key == "" {
		return dcache.ErrInvalidKey
	}

	// Store the value using the base cache.
	if err := inv.base.Set(ctx, key, value, opts); err != nil {
		return err
	}

	// Associate the prefixed key with each tag using Redis Sets.
	prefixedKey := inv.base.prefixKey(key)
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		tagK := inv.base.tagKey(tag)
		if err := inv.cmd.SAdd(ctx, tagK, prefixedKey).Err(); err != nil {
			return wrapError(err)
		}
	}

	return nil
}

// InvalidateTag removes all cache keys associated with the given tag.
// Returns the number of keys deleted.
func (inv *Invalidator) InvalidateTag(ctx context.Context, tag string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if tag == "" {
		return 0, dcache.ErrInvalidKey
	}

	tagK := inv.base.tagKey(tag)

	// Retrieve all keys associated with this tag.
	members, err := inv.cmd.SMembers(ctx, tagK).Result()
	if err != nil {
		return 0, wrapError(err)
	}

	if len(members) == 0 {
		return 0, nil
	}

	// Delete all associated cache keys.
	deleted, err := inv.cmd.Del(ctx, members...).Result()
	if err != nil {
		return 0, wrapError(err)
	}

	// Remove the tag set itself.
	if err := inv.cmd.Del(ctx, tagK).Err(); err != nil {
		return deleted, wrapError(err)
	}

	return deleted, nil
}

// Publish sends an invalidation message to all subscribers via pub/sub.
// This enables distributed cache invalidation across multiple application instances.
func (inv *Invalidator) Publish(ctx context.Context, msg InvalidationMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return inv.cmd.Publish(ctx, inv.channel, data).Err()
}

// PublishKeyInvalidation is a convenience method to publish a single-key invalidation.
func (inv *Invalidator) PublishKeyInvalidation(ctx context.Context, key string) error {
	return inv.Publish(ctx, InvalidationMessage{Type: "key", Value: key})
}

// PublishPatternInvalidation is a convenience method to publish a pattern-based invalidation.
func (inv *Invalidator) PublishPatternInvalidation(ctx context.Context, pattern string) error {
	return inv.Publish(ctx, InvalidationMessage{Type: "pattern", Value: pattern})
}

// PublishTagInvalidation is a convenience method to publish a tag-based invalidation.
func (inv *Invalidator) PublishTagInvalidation(ctx context.Context, tag string) error {
	return inv.Publish(ctx, InvalidationMessage{Type: "tag", Value: tag})
}

// Subscribe starts listening for invalidation messages and applies them locally.
// It blocks until the context is cancelled or Stop is called.
// The handler callback is invoked for each received message before the invalidation
// is applied; pass nil to skip the callback.
func (inv *Invalidator) Subscribe(ctx context.Context, handler func(InvalidationMessage)) error {
	inv.mu.Lock()
	if inv.stopCh != nil {
		inv.mu.Unlock()
		return errors.New("redis invalidator: already subscribed")
	}
	inv.stopCh = make(chan struct{})

	// We need a subscribable client. The Cmdable interface does not expose
	// Subscribe, so we attempt a type assertion to obtain a concrete client.
	var pubsub *redis.PubSub
	switch c := inv.cmd.(type) {
	case *redis.Client:
		pubsub = c.Subscribe(ctx, inv.channel)
	case *redis.ClusterClient:
		pubsub = c.Subscribe(ctx, inv.channel)
	default:
		inv.stopCh = nil
		inv.mu.Unlock()
		return errors.New("redis invalidator: unsupported client type for pub/sub")
	}
	inv.pubsub = pubsub
	stopCh := inv.stopCh
	inv.mu.Unlock()

	defer func() {
		_ = pubsub.Close()
		inv.mu.Lock()
		inv.pubsub = nil
		inv.stopCh = nil
		inv.mu.Unlock()
	}()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-stopCh:
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			var imsg InvalidationMessage
			if err := json.Unmarshal([]byte(msg.Payload), &imsg); err != nil {
				continue // skip malformed messages
			}

			if handler != nil {
				handler(imsg)
			}

			inv.applyInvalidation(ctx, imsg)
		}
	}
}

// Stop stops the subscription loop started by Subscribe.
func (inv *Invalidator) Stop() {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	if inv.stopCh != nil {
		close(inv.stopCh)
	}
}

// applyInvalidation executes a local invalidation based on the message type.
func (inv *Invalidator) applyInvalidation(ctx context.Context, msg InvalidationMessage) {
	switch msg.Type {
	case "key":
		_ = inv.base.Delete(ctx, msg.Value)
	case "pattern":
		_, _ = inv.InvalidatePattern(ctx, msg.Value)
	case "tag":
		_, _ = inv.InvalidateTag(ctx, msg.Value)
	}
}

// deleteByPattern scans for keys matching the full pattern and deletes them.
func (inv *Invalidator) deleteByPattern(ctx context.Context, pattern string) (int64, error) {
	// Use the concrete client type for SCAN.
	switch c := inv.cmd.(type) {
	case *redis.Client:
		return inv.scanAndCount(ctx, c, pattern)
	case *redis.ClusterClient:
		var total int64
		err := c.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
			n, err := inv.scanAndCount(ctx, node, pattern)
			if err != nil {
				return err
			}
			total += n
			return nil
		})
		return total, err
	default:
		return 0, errors.New("redis invalidator: unsupported client type for pattern scan")
	}
}

// scanAndCount scans for keys matching pattern, deletes them, and returns the count.
func (inv *Invalidator) scanAndCount(ctx context.Context, client *redis.Client, pattern string) (int64, error) {
	var total int64
	iter := client.Scan(ctx, 0, pattern, 100).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 100 {
			n, err := client.Del(ctx, keys...).Result()
			if err != nil {
				return total, wrapError(err)
			}
			total += n
			keys = keys[:0]
		}
	}

	if err := iter.Err(); err != nil {
		return total, wrapError(err)
	}

	if len(keys) > 0 {
		n, err := client.Del(ctx, keys...).Result()
		if err != nil {
			return total, wrapError(err)
		}
		total += n
	}

	return total, nil
}
