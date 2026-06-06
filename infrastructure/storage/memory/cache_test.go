package memory_test

import (
	"context"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/cache"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

func TestNewCache(t *testing.T) {
	t.Parallel()

	t.Run("creates cache with defaults", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		if c == nil {
			t.Fatal("NewCache() returned nil")
		}

		stats := c.Stats()
		if stats.MaxSize != 1000 {
			t.Errorf("default MaxSize = %d, want 1000", stats.MaxSize)
		}
	})

	t.Run("creates cache with custom max size", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache(memory.WithMaxSize(500))
		stats := c.Stats()
		if stats.MaxSize != 500 {
			t.Errorf("MaxSize = %d, want 500", stats.MaxSize)
		}
	})
}

func TestCache_SetAndGet(t *testing.T) {
	t.Parallel()

	t.Run("sets and gets value", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx := context.Background()

		err := c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		value, found, err := c.Get(ctx, "key1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if !found {
			t.Error("Get() should find the key")
		}
		if string(value) != "value1" {
			t.Errorf("Get() value = %s, want value1", value)
		}
	})

	t.Run("returns miss for non-existent key", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx := context.Background()

		_, found, err := c.Get(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if found {
			t.Error("Get() should not find non-existent key")
		}
	})

	t.Run("respects TTL expiration", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx := context.Background()

		err := c.Set(ctx, "expiring", []byte("value"), cache.SetOptions{TTL: 50 * time.Millisecond})
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Should exist immediately
		_, found, _ := c.Get(ctx, "expiring")
		if !found {
			t.Error("Key should exist before expiration")
		}

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Should be expired
		_, found, _ = c.Get(ctx, "expiring")
		if found {
			t.Error("Key should be expired")
		}
	})

	t.Run("returns error for empty key", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx := context.Background()

		err := c.Set(ctx, "", []byte("value"), cache.SetOptions{})
		if err != cache.ErrInvalidKey {
			t.Errorf("Set() error = %v, want ErrInvalidKey", err)
		}
	})

	t.Run("returns error for cancelled context on Set", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := c.Set(ctx, "key", []byte("value"), cache.SetOptions{})
		if err == nil {
			t.Error("Set() should return error for cancelled context")
		}
	})

	t.Run("returns error for cancelled context on Get", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, _, err := c.Get(ctx, "key")
		if err == nil {
			t.Error("Get() should return error for cancelled context")
		}
	})
}

func TestCache_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing key", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx := context.Background()

		c.Set(ctx, "key", []byte("value"), cache.SetOptions{})

		err := c.Delete(ctx, "key")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, found, _ := c.Get(ctx, "key")
		if found {
			t.Error("Key should be deleted")
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := c.Delete(ctx, "key")
		if err == nil {
			t.Error("Delete() should return error for cancelled context")
		}
	})
}

func TestCache_Exists(t *testing.T) {
	t.Parallel()

	t.Run("returns true for existing key", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx := context.Background()

		c.Set(ctx, "key", []byte("value"), cache.SetOptions{})

		exists, err := c.Exists(ctx, "key")
		if err != nil {
			t.Fatalf("Exists() error = %v", err)
		}
		if !exists {
			t.Error("Exists() should return true for existing key")
		}
	})

	t.Run("returns false for expired key", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx := context.Background()

		c.Set(ctx, "key", []byte("value"), cache.SetOptions{TTL: 10 * time.Millisecond})
		time.Sleep(50 * time.Millisecond)

		exists, err := c.Exists(ctx, "key")
		if err != nil {
			t.Fatalf("Exists() error = %v", err)
		}
		if exists {
			t.Error("Exists() should return false for expired key")
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := c.Exists(ctx, "key")
		if err == nil {
			t.Error("Exists() should return error for cancelled context")
		}
	})
}

func TestCache_Clear(t *testing.T) {
	t.Parallel()

	t.Run("clears all entries", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx := context.Background()

		c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})
		c.Set(ctx, "key2", []byte("value2"), cache.SetOptions{})

		err := c.Clear(ctx)
		if err != nil {
			t.Fatalf("Clear() error = %v", err)
		}

		if c.Size() != 0 {
			t.Errorf("Size() = %d, want 0 after Clear()", c.Size())
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		c := memory.NewCache()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := c.Clear(ctx)
		if err == nil {
			t.Error("Clear() should return error for cancelled context")
		}
	})
}

func TestCache_Stats(t *testing.T) {
	t.Parallel()

	c := memory.NewCache()
	ctx := context.Background()

	// Set and get to generate hits/misses
	c.Set(ctx, "key", []byte("value"), cache.SetOptions{})
	c.Get(ctx, "key")         // Hit
	c.Get(ctx, "nonexistent") // Miss

	stats := c.Stats()
	if stats.Hits != 1 {
		t.Errorf("Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
	if stats.Size != 1 {
		t.Errorf("Size = %d, want 1", stats.Size)
	}
}

func TestCache_LRUEviction(t *testing.T) {
	t.Parallel()

	c := memory.NewCache(memory.WithMaxSize(2))
	ctx := context.Background()

	// Fill cache
	c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})
	time.Sleep(10 * time.Millisecond) // Ensure different access times
	c.Set(ctx, "key2", []byte("value2"), cache.SetOptions{})

	// Access key1 to make it more recent
	c.Get(ctx, "key1")
	time.Sleep(10 * time.Millisecond)

	// Add another entry, should evict key2 (LRU)
	c.Set(ctx, "key3", []byte("value3"), cache.SetOptions{})

	// key1 should still exist (was accessed)
	_, found, _ := c.Get(ctx, "key1")
	if !found {
		t.Error("key1 should still exist (was accessed recently)")
	}

	// key3 should exist
	_, found, _ = c.Get(ctx, "key3")
	if !found {
		t.Error("key3 should exist")
	}
}

func TestCache_Cleanup(t *testing.T) {
	t.Parallel()

	c := memory.NewCache()
	ctx := context.Background()

	// Add entries with short TTL
	c.Set(ctx, "expiring1", []byte("value"), cache.SetOptions{TTL: 10 * time.Millisecond})
	c.Set(ctx, "expiring2", []byte("value"), cache.SetOptions{TTL: 10 * time.Millisecond})
	c.Set(ctx, "permanent", []byte("value"), cache.SetOptions{})

	// Wait for expiration
	time.Sleep(50 * time.Millisecond)

	// Cleanup should remove 2 expired entries
	removed := c.Cleanup()
	if removed != 2 {
		t.Errorf("Cleanup() removed = %d, want 2", removed)
	}

	// Permanent entry should still exist
	_, found, _ := c.Get(ctx, "permanent")
	if !found {
		t.Error("Permanent entry should still exist")
	}
}

func TestCache_Size(t *testing.T) {
	t.Parallel()

	c := memory.NewCache()
	ctx := context.Background()

	if c.Size() != 0 {
		t.Errorf("Size() = %d, want 0 for empty cache", c.Size())
	}

	c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})
	c.Set(ctx, "key2", []byte("value2"), cache.SetOptions{})

	if c.Size() != 2 {
		t.Errorf("Size() = %d, want 2", c.Size())
	}
}

func TestCache_CacheFull(t *testing.T) {
	t.Parallel()

	// Create a very small cache
	c := memory.NewCache(memory.WithMaxSize(1))
	ctx := context.Background()

	// Fill it
	c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})

	// Second entry should trigger eviction but still succeed
	err := c.Set(ctx, "key2", []byte("value2"), cache.SetOptions{})
	if err != nil {
		t.Errorf("Set() should succeed after eviction, got error = %v", err)
	}
}
