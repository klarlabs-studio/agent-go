package etcd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/cache"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

// mockKV implements a mock etcd KV interface for testing
type mockKV struct {
	getData    map[string][]byte
	getErr     error
	putErr     error
	deleteErr  error
	grantErr   error
	leaseID    clientv3.LeaseID
}

func (m *mockKV) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}

	value, exists := m.getData[key]
	if !exists {
		return &clientv3.GetResponse{Count: 0}, nil
	}

	return &clientv3.GetResponse{
		Count: 1,
		Kvs: []*mvccpb.KeyValue{
			{Key: []byte(key), Value: value},
		},
	}, nil
}

func (m *mockKV) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}

	if m.getData == nil {
		m.getData = make(map[string][]byte)
	}
	m.getData[key] = []byte(val)

	return &clientv3.PutResponse{}, nil
}

func (m *mockKV) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}

	delete(m.getData, key)
	return &clientv3.DeleteResponse{}, nil
}

func (m *mockKV) Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	if m.grantErr != nil {
		return nil, m.grantErr
	}

	return &clientv3.LeaseGrantResponse{
		ID:  m.leaseID,
		TTL: ttl,
	}, nil
}

func TestCache_InterfaceCompliance(t *testing.T) {
	// Verify Cache implements cache.Cache interface
	var _ cache.Cache = (*Cache)(nil)
}

func TestNewCache(t *testing.T) {
	client := &clientv3.Client{}
	c := NewCache(client)

	if c == nil {
		t.Fatal("expected cache instance, got nil")
	}

	if c.client != client {
		t.Error("client not set correctly")
	}

	if c.keyPrefix != "agent/cache/" {
		t.Errorf("expected default prefix 'agent/cache/', got %s", c.keyPrefix)
	}
}

func TestNewCacheWithConfig(t *testing.T) {
	client := &clientv3.Client{}

	tests := []struct {
		name           string
		config         CacheConfig
		expectedPrefix string
	}{
		{
			name:           "custom prefix",
			config:         CacheConfig{KeyPrefix: "custom/"},
			expectedPrefix: "custom/",
		},
		{
			name:           "empty prefix uses default",
			config:         CacheConfig{KeyPrefix: ""},
			expectedPrefix: "agent/cache/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCacheWithConfig(client, tt.config)
			if c.keyPrefix != tt.expectedPrefix {
				t.Errorf("expected prefix %s, got %s", tt.expectedPrefix, c.keyPrefix)
			}
		})
	}
}

func TestCache_PrefixKey(t *testing.T) {
	c := &Cache{keyPrefix: "test/"}

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "simple key",
			key:      "mykey",
			expected: "test/mykey",
		},
		{
			name:     "empty key",
			key:      "",
			expected: "test/",
		},
		{
			name:     "key with slashes",
			key:      "my/nested/key",
			expected: "test/my/nested/key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.prefixKey(tt.key)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCache_Set_InvalidKey(t *testing.T) {
	c := &Cache{
		client:    &clientv3.Client{},
		keyPrefix: "test/",
	}

	err := c.Set(context.Background(), "", []byte("value"), cache.SetOptions{})
	if !errors.Is(err, cache.ErrInvalidKey) {
		t.Errorf("expected ErrInvalidKey, got %v", err)
	}
}

func TestWrapError(t *testing.T) {
	tests := []struct {
		name        string
		inputErr    error
		expectedErr error
	}{
		{
			name:        "nil error",
			inputErr:    nil,
			expectedErr: nil,
		},
		{
			name:        "deadline exceeded",
			inputErr:    context.DeadlineExceeded,
			expectedErr: cache.ErrOperationTimeout,
		},
		{
			name:        "other error",
			inputErr:    errors.New("some error"),
			expectedErr: cache.ErrConnectionFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapError(tt.inputErr)

			if tt.expectedErr == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if !errors.Is(result, tt.expectedErr) {
				t.Errorf("expected error wrapping %v, got %v", tt.expectedErr, result)
			}
		})
	}
}

func TestCache_ContextCancellation(t *testing.T) {
	c := &Cache{
		client:    &clientv3.Client{},
		keyPrefix: "test/",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	t.Run("Get", func(t *testing.T) {
		_, _, err := c.Get(ctx, "key")
		if err == nil {
			t.Error("expected error for canceled context")
		}
	})

	t.Run("Set", func(t *testing.T) {
		err := c.Set(ctx, "key", []byte("value"), cache.SetOptions{})
		if err == nil {
			t.Error("expected error for canceled context")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := c.Delete(ctx, "key")
		if err == nil {
			t.Error("expected error for canceled context")
		}
	})

	t.Run("Exists", func(t *testing.T) {
		_, err := c.Exists(ctx, "key")
		if err == nil {
			t.Error("expected error for canceled context")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		err := c.Clear(ctx)
		if err == nil {
			t.Error("expected error for canceled context")
		}
	})
}

// Integration test - requires running etcd server
// Run with: go test -tags=integration
func TestCache_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Try to connect to etcd
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Skipf("etcd not available: %v", err)
	}
	defer client.Close()

	// Create cache
	c := NewCache(client)
	ctx := context.Background()

	// Clear any existing test data
	_ = c.Clear(ctx)

	t.Run("Set and Get", func(t *testing.T) {
		key := "test-key"
		value := []byte("test-value")

		err := c.Set(ctx, key, value, cache.SetOptions{})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		got, found, err := c.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !found {
			t.Fatal("key not found")
		}
		if string(got) != string(value) {
			t.Errorf("expected %s, got %s", value, got)
		}
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		_, found, err := c.Get(ctx, "non-existent")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if found {
			t.Error("expected key not to be found")
		}
	})

	t.Run("Exists", func(t *testing.T) {
		key := "exists-test"
		value := []byte("value")

		err := c.Set(ctx, key, value, cache.SetOptions{})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		exists, err := c.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("expected key to exist")
		}

		exists, err = c.Exists(ctx, "non-existent")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("expected key not to exist")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		key := "delete-test"
		value := []byte("value")

		err := c.Set(ctx, key, value, cache.SetOptions{})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		err = c.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		exists, err := c.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("expected key not to exist after delete")
		}
	})

	t.Run("Set with TTL", func(t *testing.T) {
		key := "ttl-test"
		value := []byte("value")

		err := c.Set(ctx, key, value, cache.SetOptions{TTL: 1 * time.Second})
		if err != nil {
			t.Fatalf("Set with TTL failed: %v", err)
		}

		// Verify it exists immediately
		exists, err := c.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("expected key to exist")
		}

		// Wait for expiration
		time.Sleep(2 * time.Second)

		// Verify it's gone
		exists, err = c.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("expected key to be expired")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		// Set multiple keys
		for i := 0; i < 5; i++ {
			key := "clear-test-" + string(rune('a'+i))
			err := c.Set(ctx, key, []byte("value"), cache.SetOptions{})
			if err != nil {
				t.Fatalf("Set failed: %v", err)
			}
		}

		// Clear all
		err := c.Clear(ctx)
		if err != nil {
			t.Fatalf("Clear failed: %v", err)
		}

		// Verify all gone
		for i := 0; i < 5; i++ {
			key := "clear-test-" + string(rune('a'+i))
			exists, err := c.Exists(ctx, key)
			if err != nil {
				t.Fatalf("Exists failed: %v", err)
			}
			if exists {
				t.Errorf("expected key %s to be cleared", key)
			}
		}
	})
}
