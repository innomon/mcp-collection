package main

import (
	"sync"
	"testing"
	"time"
)

func TestCacheSetAndGet(t *testing.T) {
	c := NewDocTypeCache(5 * time.Minute)
	dt := &FrappeDocType{Name: "Customer", Module: "Selling"}

	c.Set("Customer", dt)

	got, ok := c.Get("Customer")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Name != "Customer" {
		t.Errorf("name = %q, want Customer", got.Name)
	}
}

func TestCacheMiss(t *testing.T) {
	c := NewDocTypeCache(5 * time.Minute)

	_, ok := c.Get("Nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	now := time.Now()
	c := NewDocTypeCache(1 * time.Second)
	c.now = func() time.Time { return now }

	dt := &FrappeDocType{Name: "Customer"}
	c.Set("Customer", dt)

	// Still valid
	_, ok := c.Get("Customer")
	if !ok {
		t.Fatal("expected cache hit before expiry")
	}

	// Advance past TTL
	c.now = func() time.Time { return now.Add(2 * time.Second) }
	_, ok = c.Get("Customer")
	if ok {
		t.Error("expected cache miss after TTL expiry")
	}
}

func TestCacheConcurrency(t *testing.T) {
	c := NewDocTypeCache(5 * time.Minute)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.Set("Customer", &FrappeDocType{Name: "Customer"})
		}()
		go func() {
			defer wg.Done()
			c.Get("Customer")
		}()
	}
	wg.Wait()
}
