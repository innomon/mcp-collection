package main

import "testing"

func TestMetricsCollectorIncCounter(t *testing.T) {
	c := NewMetricsCollector()
	c.IncCounter("test_total", nil)
	c.IncCounter("test_total", nil)
	c.IncCounter("test_total", nil)

	snap := c.Snapshot()
	if snap["test_total"].Count != 3 {
		t.Errorf("count = %d, want 3", snap["test_total"].Count)
	}
}

func TestMetricsCollectorObserveLatency(t *testing.T) {
	c := NewMetricsCollector()
	c.ObserveLatency("test_latency", nil, 100)
	c.ObserveLatency("test_latency", nil, 200)

	snap := c.Snapshot()
	v := snap["test_latency"]
	if v.LatencyCount != 2 {
		t.Errorf("latency_count = %d, want 2", v.LatencyCount)
	}
	if v.LatencySum != 300 {
		t.Errorf("latency_sum = %d, want 300", v.LatencySum)
	}
}

func TestMetricsCollectorLabels(t *testing.T) {
	c := NewMetricsCollector()
	c.IncCounter("test", map[string]string{"a": "1", "b": "2"})

	snap := c.Snapshot()
	if _, ok := snap["test{a=1,b=2}"]; !ok {
		t.Error("expected labeled metric key test{a=1,b=2}")
	}
}

func TestMetricsCollectorSnapshot(t *testing.T) {
	c := NewMetricsCollector()
	c.IncCounter("x", nil)

	snap1 := c.Snapshot()
	c.IncCounter("x", nil)
	snap2 := c.Snapshot()

	if snap1["x"].Count != 1 {
		t.Errorf("snap1 count = %d, want 1", snap1["x"].Count)
	}
	if snap2["x"].Count != 2 {
		t.Errorf("snap2 count = %d, want 2", snap2["x"].Count)
	}
}
