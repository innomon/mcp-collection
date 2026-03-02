package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	metricDocTypeFetchTotal       = "doctype_fetch_total"
	metricDocTypeFetchLatencyMs   = "doctype_fetch_latency_ms"
	metricDocTypeCacheHitTotal    = "doctype_cache_hit_total"
	metricDocTypeCacheMissTotal   = "doctype_cache_miss_total"
	metricSchemaAssemblyLatencyMs = "schema_assembly_latency_ms"
	metricSchemaPipelineLatencyMs = "schema_pipeline_latency_ms"
	metricSchemaFallbackTotal     = "schema_fallback_total"
)

// MetricValue holds the aggregated data for a single metric key.
type MetricValue struct {
	Count        int64
	LatencySum   int64
	LatencyCount int64
}

// MetricsCollector stores counters and latency observations in memory.
type MetricsCollector struct {
	mu      sync.Mutex
	metrics map[string]MetricValue
}

// NewMetricsCollector returns a ready-to-use MetricsCollector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]MetricValue),
	}
}

// IncCounter increments the counter identified by name and labels by one.
func (c *MetricsCollector) IncCounter(name string, labels map[string]string) {
	key := formatMetricKey(name, labels)

	c.mu.Lock()
	defer c.mu.Unlock()

	v := c.metrics[key]
	v.Count++
	c.metrics[key] = v
}

// ObserveLatency records a latency observation in milliseconds.
func (c *MetricsCollector) ObserveLatency(name string, labels map[string]string, durationMs int64) {
	key := formatMetricKey(name, labels)

	c.mu.Lock()
	defer c.mu.Unlock()

	v := c.metrics[key]
	v.LatencySum += durationMs
	v.LatencyCount++
	c.metrics[key] = v
}

// Snapshot returns a point-in-time copy of all metric values.
func (c *MetricsCollector) Snapshot() map[string]MetricValue {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make(map[string]MetricValue, len(c.metrics))
	for k, v := range c.metrics {
		out[k] = v
	}
	return out
}

func formatMetricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}

	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, labels[k]))
	}

	return name + "{" + strings.Join(pairs, ",") + "}"
}
