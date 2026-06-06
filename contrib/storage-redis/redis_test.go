package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"go.klarlabs.de/agent/domain/cache"
)

// newTestCache creates a Cache backed by miniredis for unit testing.
func newTestCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	c := NewCacheFromClient(client, "test:")
	return c, mr
}

// ---------------------------------------------------------------------------
// Cache basic operations
// ---------------------------------------------------------------------------

func TestCache_GetSetDelete(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()

	// Get non-existent key returns not-found.
	val, found, err := c.Get(ctx, "missing")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if found {
		t.Error("expected not found for missing key")
	}
	if val != nil {
		t.Error("expected nil value for missing key")
	}

	// Set a value.
	if err := c.Set(ctx, "k1", []byte("v1"), cache.SetOptions{}); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Get the value back.
	val, found, err = c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !found {
		t.Fatal("expected key to be found")
	}
	if string(val) != "v1" {
		t.Fatalf("expected v1, got %s", val)
	}

	// Delete the value.
	if err := c.Delete(ctx, "k1"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	// Confirm deletion.
	_, found, err = c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get after delete error: %v", err)
	}
	if found {
		t.Error("expected key to be deleted")
	}
}

func TestCache_SetEmptyKey(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()

	err := c.Set(ctx, "", []byte("value"), cache.SetOptions{})
	if err != cache.ErrInvalidKey {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestCache_Exists(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()

	exists, err := c.Exists(ctx, "nope")
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if exists {
		t.Error("expected key to not exist")
	}

	_ = c.Set(ctx, "present", []byte("yes"), cache.SetOptions{})

	exists, err = c.Exists(ctx, "present")
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Error("expected key to exist")
	}
}

func TestCache_TTL(t *testing.T) {
	c, mr := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "expiring", []byte("data"), cache.SetOptions{TTL: 10 * time.Second})

	// Fast-forward miniredis time.
	mr.FastForward(11 * time.Second)

	_, found, err := c.Get(ctx, "expiring")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if found {
		t.Error("expected key to have expired")
	}
}

func TestCache_Clear(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		key := "item" + string(rune('a'+i))
		_ = c.Set(ctx, key, []byte("data"), cache.SetOptions{})
	}

	if err := c.Clear(ctx); err != nil {
		t.Fatalf("Clear error: %v", err)
	}

	for i := 0; i < 5; i++ {
		key := "item" + string(rune('a'+i))
		_, found, _ := c.Get(ctx, key)
		if found {
			t.Errorf("expected key %s to be cleared", key)
		}
	}
}

func TestCache_Stats(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "s1", []byte("v"), cache.SetOptions{})
	_, _, _ = c.Get(ctx, "s1") // hit
	_, _, _ = c.Get(ctx, "s2") // miss

	stats := c.Stats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestCache_CancelledContext(t *testing.T) {
	c, _ := newTestCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := c.Get(ctx, "k")
	if err == nil {
		t.Error("expected error from cancelled context")
	}

	err = c.Set(ctx, "k", []byte("v"), cache.SetOptions{})
	if err == nil {
		t.Error("expected error from cancelled context")
	}

	err = c.Delete(ctx, "k")
	if err == nil {
		t.Error("expected error from cancelled context")
	}

	_, err = c.Exists(ctx, "k")
	if err == nil {
		t.Error("expected error from cancelled context")
	}

	err = c.Clear(ctx)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestCache_Ping(t *testing.T) {
	c, _ := newTestCache(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Invalidation: pattern-based
// ---------------------------------------------------------------------------

func TestInvalidator_InvalidatePattern(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	inv := NewInvalidator(c)

	// Seed keys.
	for _, key := range []string{"user:1", "user:2", "user:3", "order:1"} {
		_ = c.Set(ctx, key, []byte("data"), cache.SetOptions{})
	}

	deleted, err := inv.InvalidatePattern(ctx, "user:*")
	if err != nil {
		t.Fatalf("InvalidatePattern error: %v", err)
	}
	if deleted != 3 {
		t.Errorf("expected 3 deleted, got %d", deleted)
	}

	// order:1 should still exist.
	_, found, _ := c.Get(ctx, "order:1")
	if !found {
		t.Error("expected order:1 to still exist")
	}
}

func TestInvalidator_InvalidatePattern_EmptyPattern(t *testing.T) {
	c, _ := newTestCache(t)
	inv := NewInvalidator(c)

	_, err := inv.InvalidatePattern(context.Background(), "")
	if err != cache.ErrInvalidKey {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Invalidation: tag-based
// ---------------------------------------------------------------------------

func TestInvalidator_TagInvalidation(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	inv := NewInvalidator(c)

	// Store keys with tags.
	_ = inv.SetWithTags(ctx, "prod:1", []byte("a"), cache.SetOptions{}, "products", "featured")
	_ = inv.SetWithTags(ctx, "prod:2", []byte("b"), cache.SetOptions{}, "products")
	_ = inv.SetWithTags(ctx, "user:1", []byte("c"), cache.SetOptions{}, "users")

	// Invalidate the "products" tag.
	deleted, err := inv.InvalidateTag(ctx, "products")
	if err != nil {
		t.Fatalf("InvalidateTag error: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	// prod:1 and prod:2 should be gone.
	_, found, _ := c.Get(ctx, "prod:1")
	if found {
		t.Error("expected prod:1 to be invalidated")
	}
	_, found, _ = c.Get(ctx, "prod:2")
	if found {
		t.Error("expected prod:2 to be invalidated")
	}

	// user:1 should still exist.
	_, found, _ = c.Get(ctx, "user:1")
	if !found {
		t.Error("expected user:1 to still exist")
	}
}

func TestInvalidator_InvalidateTag_Empty(t *testing.T) {
	c, _ := newTestCache(t)
	inv := NewInvalidator(c)

	_, err := inv.InvalidateTag(context.Background(), "")
	if err != cache.ErrInvalidKey {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestInvalidator_InvalidateTag_NonExistent(t *testing.T) {
	c, _ := newTestCache(t)
	inv := NewInvalidator(c)

	deleted, err := inv.InvalidateTag(context.Background(), "no-such-tag")
	if err != nil {
		t.Fatalf("InvalidateTag error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}
}

func TestInvalidator_SetWithTags_EmptyKey(t *testing.T) {
	c, _ := newTestCache(t)
	inv := NewInvalidator(c)

	err := inv.SetWithTags(context.Background(), "", []byte("v"), cache.SetOptions{}, "tag")
	if err != cache.ErrInvalidKey {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Invalidation: pub/sub
// ---------------------------------------------------------------------------

func TestInvalidator_PubSub(t *testing.T) {
	c, _ := newTestCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inv := NewInvalidator(c)

	// Set a key that will be invalidated via pub/sub.
	_ = c.Set(ctx, "pubsub-key", []byte("value"), cache.SetOptions{})

	received := make(chan InvalidationMessage, 1)

	// Start subscriber in background.
	subDone := make(chan error, 1)
	go func() {
		subDone <- inv.Subscribe(ctx, func(msg InvalidationMessage) {
			received <- msg
		})
	}()

	// Give subscriber time to connect.
	time.Sleep(50 * time.Millisecond)

	// Publish an invalidation.
	if err := inv.PublishKeyInvalidation(ctx, "pubsub-key"); err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	// Wait for the message.
	select {
	case msg := <-received:
		if msg.Type != "key" || msg.Value != "pubsub-key" {
			t.Errorf("unexpected message: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pub/sub message")
	}

	// The key should have been invalidated by the subscriber.
	time.Sleep(50 * time.Millisecond)
	_, found, _ := c.Get(ctx, "pubsub-key")
	if found {
		t.Error("expected pubsub-key to be invalidated by subscriber")
	}

	// Stop subscriber.
	inv.Stop()
	select {
	case err := <-subDone:
		if err != nil {
			t.Errorf("Subscribe returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Subscribe to return")
	}
}

func TestInvalidator_DoubleSubscribe(t *testing.T) {
	c, _ := newTestCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inv := NewInvalidator(c)

	done := make(chan error, 1)
	go func() {
		done <- inv.Subscribe(ctx, nil)
	}()

	time.Sleep(50 * time.Millisecond)

	// Second subscribe should fail.
	err := inv.Subscribe(ctx, nil)
	if err == nil {
		t.Error("expected error on double subscribe")
	}

	cancel()
	<-done
}

// ---------------------------------------------------------------------------
// Health checks
// ---------------------------------------------------------------------------

func TestHealthChecker_Check_Up(t *testing.T) {
	c, _ := newTestCache(t)
	hc := NewHealthChecker(c, DefaultHealthCheckConfig())

	info := hc.Check(context.Background())
	if info.Status != HealthStatusUp {
		t.Errorf("expected status up, got %s", info.Status)
	}
	if !info.Connected {
		t.Error("expected connected to be true")
	}
	if info.Latency <= 0 {
		t.Error("expected positive latency")
	}
	if info.LastCheckedAt.IsZero() {
		t.Error("expected non-zero last checked at")
	}
	if info.Error != "" {
		t.Errorf("expected no error, got %s", info.Error)
	}
}

func TestHealthChecker_Check_Down(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	addr := mr.Addr()
	mr.Close() // close immediately to simulate down server

	client := goredis.NewClient(&goredis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })

	c := NewCacheFromClient(client, "test:")
	hc := NewHealthChecker(c, DefaultHealthCheckConfig())

	info := hc.Check(context.Background())
	if info.Status != HealthStatusDown {
		t.Errorf("expected status down, got %s", info.Status)
	}
	if info.Connected {
		t.Error("expected connected to be false")
	}
	if info.Error == "" {
		t.Error("expected error message")
	}
}

func TestHealthChecker_Check_Degraded(t *testing.T) {
	c, _ := newTestCache(t)

	cfg := DefaultHealthCheckConfig()
	// Set threshold to 1 nanosecond so any real latency triggers degraded.
	cfg.DegradedLatencyThreshold = 1 * time.Nanosecond
	hc := NewHealthChecker(c, cfg)

	info := hc.Check(context.Background())
	if info.Status != HealthStatusDegraded {
		t.Errorf("expected status degraded, got %s (latency=%v)", info.Status, info.Latency)
	}
}

func TestHealthChecker_LastStatus(t *testing.T) {
	c, _ := newTestCache(t)
	hc := NewHealthChecker(c, DefaultHealthCheckConfig())

	// Before any check, LastStatus should be zero-value.
	last := hc.LastStatus()
	if last.Connected {
		t.Error("expected initial status to not be connected")
	}

	hc.Check(context.Background())
	last = hc.LastStatus()
	if !last.Connected {
		t.Error("expected connected after check")
	}
}

func TestHealthChecker_StartStop(t *testing.T) {
	c, _ := newTestCache(t)
	cfg := HealthCheckConfig{
		Interval:                 50 * time.Millisecond,
		Timeout:                  5 * time.Second,
		DegradedLatencyThreshold: 100 * time.Millisecond,
	}
	hc := NewHealthChecker(c, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hc.StartMonitoring(ctx)

	// Wait for a few checks to run.
	time.Sleep(200 * time.Millisecond)

	last := hc.LastStatus()
	if !last.Connected {
		t.Error("expected connected after monitoring")
	}

	hc.Stop()

	// Calling Stop again should be safe.
	hc.Stop()
}

func TestHealthChecker_MonitoringContextCancel(t *testing.T) {
	c, _ := newTestCache(t)
	cfg := HealthCheckConfig{
		Interval:                 50 * time.Millisecond,
		Timeout:                  5 * time.Second,
		DegradedLatencyThreshold: 100 * time.Millisecond,
	}
	hc := NewHealthChecker(c, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	hc.StartMonitoring(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()

	// Give the goroutine time to exit.
	time.Sleep(100 * time.Millisecond)

	// Should be safe to call Stop even after context cancel.
	hc.Stop()
}

// ---------------------------------------------------------------------------
// Interface compliance
// ---------------------------------------------------------------------------

func TestCacheImplementsInterface(t *testing.T) {
	// Compile-time checks are in var blocks; this test documents them.
	var _ cache.Cache = (*Cache)(nil)
	var _ cache.StatsProvider = (*Cache)(nil)
	var _ cache.Cache = (*ClusterCache)(nil)
	var _ cache.StatsProvider = (*ClusterCache)(nil)
}

// ---------------------------------------------------------------------------
// parseInfoInt
// ---------------------------------------------------------------------------

func TestParseInfoInt(t *testing.T) {
	info := "# Memory\r\nused_memory:1024000\r\nmaxmemory:2048000\r\n# Clients\r\nconnected_clients:5\r\n"

	tests := []struct {
		field string
		want  int64
	}{
		{"used_memory", 1024000},
		{"maxmemory", 2048000},
		{"connected_clients", 5},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		got := parseInfoInt(info, tt.field)
		if got != tt.want {
			t.Errorf("parseInfoInt(%q) = %d, want %d", tt.field, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// DefaultHealthCheckConfig
// ---------------------------------------------------------------------------

func TestDefaultHealthCheckConfig(t *testing.T) {
	cfg := DefaultHealthCheckConfig()
	if cfg.Interval != 30*time.Second {
		t.Errorf("expected 30s interval, got %v", cfg.Interval)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", cfg.Timeout)
	}
	if cfg.DegradedLatencyThreshold != 100*time.Millisecond {
		t.Errorf("expected 100ms threshold, got %v", cfg.DegradedLatencyThreshold)
	}
}
