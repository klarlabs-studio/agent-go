// Package cache provides in-memory caching tools for agents.
package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// CacheStore manages cached items.
type CacheStore struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
}

type cacheItem struct {
	Value       any           `json:"value"`
	ExpiresAt   time.Time     `json:"expires_at,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	AccessedAt  time.Time     `json:"accessed_at"`
	AccessCount int           `json:"access_count"`
	TTL         time.Duration `json:"-"`
}

func (i *cacheItem) isExpired() bool {
	if i.TTL == 0 {
		return false
	}
	return time.Now().After(i.ExpiresAt)
}

var store = &CacheStore{
	items: make(map[string]*cacheItem),
}

// Pack returns the cache tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("cache").
		WithDescription("In-memory caching tools").
		AddTools(
			setTool(),
			getTool(),
			deleteTool(),
			existsTool(),
			expireTool(),
			ttlTool(),
			keysTool(),
			clearTool(),
			statsTool(),
			incrTool(),
			mgetTool(),
			msetTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func setTool() tool.Tool {
	return tool.NewBuilder("cache_set").
		WithDescription("Set a value in cache").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key   string `json:"key"`
				Value any    `json:"value"`
				TTL   int    `json:"ttl_seconds,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			now := time.Now()
			item := &cacheItem{
				Value:       params.Value,
				CreatedAt:   now,
				AccessedAt:  now,
				AccessCount: 0,
			}

			if params.TTL > 0 {
				item.TTL = time.Duration(params.TTL) * time.Second
				item.ExpiresAt = now.Add(item.TTL)
			}

			store.mu.Lock()
			store.items[params.Key] = item
			store.mu.Unlock()

			result := map[string]any{
				"key": params.Key,
				"set": true,
				"ttl": params.TTL,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func getTool() tool.Tool {
	return tool.NewBuilder("cache_get").
		WithDescription("Get a value from cache").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key     string `json:"key"`
				Default any    `json:"default,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.Lock()
			item, ok := store.items[params.Key]
			if ok {
				if item.isExpired() {
					delete(store.items, params.Key)
					ok = false
				} else {
					item.AccessedAt = time.Now()
					item.AccessCount++
				}
			}
			store.mu.Unlock()

			if !ok {
				result := map[string]any{
					"key":   params.Key,
					"found": false,
					"value": params.Default,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"key":   params.Key,
				"found": true,
				"value": item.Value,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func deleteTool() tool.Tool {
	return tool.NewBuilder("cache_delete").
		WithDescription("Delete a key from cache").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.Lock()
			_, existed := store.items[params.Key]
			delete(store.items, params.Key)
			store.mu.Unlock()

			result := map[string]any{
				"key":     params.Key,
				"deleted": existed,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func existsTool() tool.Tool {
	return tool.NewBuilder("cache_exists").
		WithDescription("Check if a key exists in cache").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			item, ok := store.items[params.Key]
			exists := ok && !item.isExpired()
			store.mu.RUnlock()

			result := map[string]any{
				"key":    params.Key,
				"exists": exists,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func expireTool() tool.Tool {
	return tool.NewBuilder("cache_expire").
		WithDescription("Set expiration on a key").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key string `json:"key"`
				TTL int    `json:"ttl_seconds"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.Lock()
			item, ok := store.items[params.Key]
			if ok && !item.isExpired() {
				item.TTL = time.Duration(params.TTL) * time.Second
				item.ExpiresAt = time.Now().Add(item.TTL)
			}
			store.mu.Unlock()

			result := map[string]any{
				"key":     params.Key,
				"updated": ok,
				"ttl":     params.TTL,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func ttlTool() tool.Tool {
	return tool.NewBuilder("cache_ttl").
		WithDescription("Get remaining TTL for a key").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			item, ok := store.items[params.Key]
			var ttlSeconds int
			var expired bool

			if ok {
				switch {
				case item.TTL == 0:
					ttlSeconds = -1 // No expiry
				case item.isExpired():
					expired = true
					ttlSeconds = 0
				default:
					ttlSeconds = int(time.Until(item.ExpiresAt).Seconds())
				}
			}
			store.mu.RUnlock()

			result := map[string]any{
				"key":         params.Key,
				"exists":      ok && !expired,
				"ttl_seconds": ttlSeconds,
				"expires_at":  nil,
			}
			if ok && !expired && item.TTL > 0 {
				result["expires_at"] = item.ExpiresAt
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func keysTool() tool.Tool {
	return tool.NewBuilder("cache_keys").
		WithDescription("List all keys in cache").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern string `json:"pattern,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			var keys []string
			for key, item := range store.items {
				if item.isExpired() {
					continue
				}
				if params.Pattern != "" {
					// Simple prefix/suffix matching
					switch {
					case params.Pattern[0] == '*':
						suffix := params.Pattern[1:]
						if len(key) < len(suffix) || key[len(key)-len(suffix):] != suffix {
							continue
						}
					case params.Pattern[len(params.Pattern)-1] == '*':
						prefix := params.Pattern[:len(params.Pattern)-1]
						if len(key) < len(prefix) || key[:len(prefix)] != prefix {
							continue
						}
					case key != params.Pattern:
						continue
					}
				}
				keys = append(keys, key)
			}
			store.mu.RUnlock()

			result := map[string]any{
				"keys":  keys,
				"count": len(keys),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func clearTool() tool.Tool {
	return tool.NewBuilder("cache_clear").
		WithDescription("Clear all keys from cache").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern string `json:"pattern,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.Lock()
			var cleared int

			if params.Pattern == "" {
				cleared = len(store.items)
				store.items = make(map[string]*cacheItem)
			} else {
				for key := range store.items {
					match := false
					switch {
					case params.Pattern[0] == '*':
						suffix := params.Pattern[1:]
						match = len(key) >= len(suffix) && key[len(key)-len(suffix):] == suffix
					case params.Pattern[len(params.Pattern)-1] == '*':
						prefix := params.Pattern[:len(params.Pattern)-1]
						match = len(key) >= len(prefix) && key[:len(prefix)] == prefix
					default:
						match = key == params.Pattern
					}
					if match {
						delete(store.items, key)
						cleared++
					}
				}
			}
			store.mu.Unlock()

			result := map[string]any{
				"cleared": cleared,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func statsTool() tool.Tool {
	return tool.NewBuilder("cache_stats").
		WithDescription("Get cache statistics").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			store.mu.RLock()
			total := len(store.items)
			var expired, withTTL, permanent int
			var totalAccess int

			for _, item := range store.items {
				totalAccess += item.AccessCount
				switch {
				case item.TTL == 0:
					permanent++
				case item.isExpired():
					expired++
				default:
					withTTL++
				}
			}
			store.mu.RUnlock()

			result := map[string]any{
				"total_keys":     total,
				"active_keys":    total - expired,
				"expired_keys":   expired,
				"with_ttl":       withTTL,
				"permanent":      permanent,
				"total_accesses": totalAccess,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func incrTool() tool.Tool {
	return tool.NewBuilder("cache_incr").
		WithDescription("Increment a numeric value in cache").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key   string `json:"key"`
				Delta int    `json:"delta,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			delta := params.Delta
			if delta == 0 {
				delta = 1
			}

			store.mu.Lock()
			item, ok := store.items[params.Key]

			var newValue int
			if !ok || item.isExpired() {
				newValue = delta
				store.items[params.Key] = &cacheItem{
					Value:      newValue,
					CreatedAt:  time.Now(),
					AccessedAt: time.Now(),
				}
			} else {
				switch v := item.Value.(type) {
				case int:
					newValue = v + delta
				case float64:
					newValue = int(v) + delta
				case json.Number:
					n, _ := v.Int64()
					newValue = int(n) + delta
				default:
					store.mu.Unlock()
					result := map[string]any{
						"key":   params.Key,
						"error": "value is not numeric",
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
				item.Value = newValue
				item.AccessedAt = time.Now()
				item.AccessCount++
			}
			store.mu.Unlock()

			result := map[string]any{
				"key":   params.Key,
				"value": newValue,
				"delta": delta,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func mgetTool() tool.Tool {
	return tool.NewBuilder("cache_mget").
		WithDescription("Get multiple values from cache").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Keys []string `json:"keys"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.Lock()
			values := make(map[string]any)
			found := 0

			for _, key := range params.Keys {
				item, ok := store.items[key]
				if ok && !item.isExpired() {
					values[key] = item.Value
					item.AccessedAt = time.Now()
					item.AccessCount++
					found++
				} else {
					values[key] = nil
				}
			}
			store.mu.Unlock()

			result := map[string]any{
				"values":    values,
				"found":     found,
				"requested": len(params.Keys),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func msetTool() tool.Tool {
	return tool.NewBuilder("cache_mset").
		WithDescription("Set multiple values in cache").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items map[string]any `json:"items"`
				TTL   int            `json:"ttl_seconds,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			now := time.Now()
			var ttl time.Duration
			var expiresAt time.Time
			if params.TTL > 0 {
				ttl = time.Duration(params.TTL) * time.Second
				expiresAt = now.Add(ttl)
			}

			store.mu.Lock()
			for key, value := range params.Items {
				store.items[key] = &cacheItem{
					Value:      value,
					CreatedAt:  now,
					AccessedAt: now,
					TTL:        ttl,
					ExpiresAt:  expiresAt,
				}
			}
			store.mu.Unlock()

			result := map[string]any{
				"set": len(params.Items),
				"ttl": params.TTL,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
