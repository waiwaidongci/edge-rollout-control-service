package api

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MetricType string

const (
	MetricCounter   MetricType = "counter"
	MetricGauge     MetricType = "gauge"
	MetricHistogram MetricType = "histogram"
)

type MetricDescriptor struct {
	Name       string
	Help       string
	Type       MetricType
	LabelNames []string
}

type metricKey struct {
	name   string
	labels string
}

type MetricRegistry struct {
	mu          sync.RWMutex
	descriptors map[string]MetricDescriptor
	values      map[metricKey]float64
	histograms  map[metricKey]*histogramValue
}

type histogramValue struct {
	Buckets []float64
	Counts  []uint64
	Count   uint64
	Sum     float64
}

func NewMetricRegistry() *MetricRegistry {
	return &MetricRegistry{descriptors: map[string]MetricDescriptor{}, values: map[metricKey]float64{}, histograms: map[metricKey]*histogramValue{}}
}

func (r *MetricRegistry) Register(descriptor MetricDescriptor) error {
	if err := validateMetricName(descriptor.Name); err != nil {
		return err
	}
	if descriptor.Help == "" {
		return fmt.Errorf("metric %s requires help text", descriptor.Name)
	}
	if descriptor.Type != MetricCounter && descriptor.Type != MetricGauge && descriptor.Type != MetricHistogram {
		return fmt.Errorf("metric %s has invalid type", descriptor.Name)
	}
	for _, label := range descriptor.LabelNames {
		if err := validateMetricName(label); err != nil {
			return fmt.Errorf("metric %s label: %w", descriptor.Name, err)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.descriptors[descriptor.Name]; exists {
		return fmt.Errorf("metric %s is already registered", descriptor.Name)
	}
	descriptor.LabelNames = append([]string(nil), descriptor.LabelNames...)
	sort.Strings(descriptor.LabelNames)
	r.descriptors[descriptor.Name] = descriptor
	return nil
}

func (r *MetricRegistry) Add(name string, labels map[string]string, delta float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	descriptor, ok := r.descriptors[name]
	if !ok {
		return fmt.Errorf("metric %s is not registered", name)
	}
	if descriptor.Type == MetricHistogram {
		return fmt.Errorf("histogram %s requires Observe", name)
	}
	if descriptor.Type == MetricCounter && delta < 0 {
		return fmt.Errorf("counter %s cannot decrease", name)
	}
	labelKey, err := encodeMetricLabels(descriptor.LabelNames, labels)
	if err != nil {
		return err
	}
	key := metricKey{name: name, labels: labelKey}
	r.values[key] += delta
	return nil
}

func (r *MetricRegistry) Set(name string, labels map[string]string, value float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	descriptor, ok := r.descriptors[name]
	if !ok {
		return fmt.Errorf("metric %s is not registered", name)
	}
	if descriptor.Type != MetricGauge {
		return fmt.Errorf("metric %s is not a gauge", name)
	}
	labelKey, err := encodeMetricLabels(descriptor.LabelNames, labels)
	if err != nil {
		return err
	}
	r.values[metricKey{name: name, labels: labelKey}] = value
	return nil
}

func (r *MetricRegistry) ConfigureHistogram(name string, buckets []float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	descriptor, ok := r.descriptors[name]
	if !ok || descriptor.Type != MetricHistogram {
		return fmt.Errorf("metric %s is not a registered histogram", name)
	}
	if len(buckets) == 0 {
		return fmt.Errorf("histogram %s requires buckets", name)
	}
	sorted := append([]float64(nil), buckets...)
	sort.Float64s(sorted)
	for index := 1; index < len(sorted); index++ {
		if sorted[index] == sorted[index-1] {
			return fmt.Errorf("histogram %s contains duplicate buckets", name)
		}
	}
	r.descriptors[name] = descriptor
	r.histograms[metricKey{name: name}] = &histogramValue{Buckets: sorted, Counts: make([]uint64, len(sorted))}
	return nil
}

func (r *MetricRegistry) Observe(name string, labels map[string]string, value float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	descriptor, ok := r.descriptors[name]
	if !ok || descriptor.Type != MetricHistogram {
		return fmt.Errorf("metric %s is not a registered histogram", name)
	}
	labelKey, err := encodeMetricLabels(descriptor.LabelNames, labels)
	if err != nil {
		return err
	}
	template := r.histograms[metricKey{name: name}]
	if template == nil {
		return fmt.Errorf("histogram %s has no configured buckets", name)
	}
	key := metricKey{name: name, labels: labelKey}
	histogram := r.histograms[key]
	if histogram == nil {
		histogram = &histogramValue{Buckets: append([]float64(nil), template.Buckets...), Counts: make([]uint64, len(template.Buckets))}
		r.histograms[key] = histogram
	}
	for index, bucket := range histogram.Buckets {
		if value <= bucket {
			histogram.Counts[index]++
		}
	}
	histogram.Count++
	histogram.Sum += value
	return nil
}

func (r *MetricRegistry) Render(writer io.Writer) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.descriptors))
	for name := range r.descriptors {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		descriptor := r.descriptors[name]
		if _, err := fmt.Fprintf(writer, "# HELP %s %s\n# TYPE %s %s\n", descriptor.Name, escapeMetricHelp(descriptor.Help), descriptor.Name, descriptor.Type); err != nil {
			return err
		}
		if descriptor.Type == MetricHistogram {
			if err := r.renderHistogram(writer, descriptor); err != nil {
				return err
			}
			continue
		}
		keys := make([]metricKey, 0)
		for key := range r.values {
			if key.name == name {
				keys = append(keys, key)
			}
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i].labels < keys[j].labels })
		for _, key := range keys {
			if _, err := fmt.Fprintf(writer, "%s%s %s\n", name, key.labels, formatMetricFloat(r.values[key])); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *MetricRegistry) Snapshot() map[string]float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]float64, len(r.values))
	for key, value := range r.values {
		result[key.name+key.labels] = value
	}
	return result
}

func (r *MetricRegistry) renderHistogram(writer io.Writer, descriptor MetricDescriptor) error {
	keys := make([]metricKey, 0)
	for key := range r.histograms {
		if key.name == descriptor.Name && key.labels != "" {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].labels < keys[j].labels })
	for _, key := range keys {
		histogram := r.histograms[key]
		for index, bucket := range histogram.Buckets {
			labels := appendMetricLabel(key.labels, "le", formatMetricFloat(bucket))
			if _, err := fmt.Fprintf(writer, "%s_bucket%s %d\n", descriptor.Name, labels, histogram.Counts[index]); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(writer, "%s_bucket%s %d\n", descriptor.Name, appendMetricLabel(key.labels, "le", "+Inf"), histogram.Count); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "%s_sum%s %s\n", descriptor.Name, key.labels, formatMetricFloat(histogram.Sum)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "%s_count%s %d\n", descriptor.Name, key.labels, histogram.Count); err != nil {
			return err
		}
	}
	return nil
}

func encodeMetricLabels(names []string, labels map[string]string) (string, error) {
	if len(names) != len(labels) {
		return "", fmt.Errorf("expected %d metric labels, got %d", len(names), len(labels))
	}
	if len(names) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(names))
	for _, name := range names {
		value, ok := labels[name]
		if !ok {
			return "", fmt.Errorf("missing metric label %s", name)
		}
		parts = append(parts, name+"=\""+escapeMetricLabel(value)+"\"")
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func appendMetricLabel(encoded, name, value string) string {
	entry := name + "=\"" + escapeMetricLabel(value) + "\""
	if encoded == "" {
		return "{" + entry + "}"
	}
	return strings.TrimSuffix(encoded, "}") + "," + entry + "}"
}

func validateMetricName(name string) error {
	if name == "" {
		return fmt.Errorf("metric name is required")
	}
	for index, character := range name {
		valid := character == '_' || character == ':' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || index > 0 && character >= '0' && character <= '9'
		if !valid {
			return fmt.Errorf("metric name %q is invalid", name)
		}
	}
	return nil
}

func escapeMetricLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return strings.ReplaceAll(value, "\"", "\\\"")
}

func escapeMetricHelp(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, "\n", "\\n")
}

func formatMetricFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

type Timer struct {
	registry *MetricRegistry
	name     string
	labels   map[string]string
	started  time.Time
}

func (r *MetricRegistry) StartTimer(name string, labels map[string]string) Timer {
	copyLabels := make(map[string]string, len(labels))
	for key, value := range labels {
		copyLabels[key] = value
	}
	return Timer{registry: r, name: name, labels: copyLabels, started: time.Now()}
}

func (t Timer) ObserveDuration() error {
	return t.registry.Observe(t.name, t.labels, time.Since(t.started).Seconds())
}
