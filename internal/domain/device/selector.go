package device

import (
	"fmt"
	"sort"
	"strings"
)

type SelectorExpression struct {
	Terms []Selector
}

func ParseSelectorExpression(raw string) (SelectorExpression, error) {
	parts := strings.Split(raw, ",")
	if len(parts) == 0 {
		return SelectorExpression{}, ErrInvalid
	}
	result := SelectorExpression{Terms: make([]Selector, 0, len(parts))}
	for _, part := range parts {
		selector, err := ParseSelector(part)
		if err != nil {
			return SelectorExpression{}, fmt.Errorf("parse selector %q: %w", part, err)
		}
		result.Terms = append(result.Terms, selector)
	}
	return result, nil
}

func (s SelectorExpression) Match(labels map[string]string) bool {
	if len(s.Terms) == 0 {
		return false
	}
	for _, term := range s.Terms {
		if labels[term.Key] != term.Value {
			return false
		}
	}
	return true
}

func (s SelectorExpression) String() string {
	terms := make([]string, 0, len(s.Terms))
	for _, term := range s.Terms {
		terms = append(terms, term.Key+"="+term.Value)
	}
	return strings.Join(terms, ",")
}

func NormalizeLabels(input map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			return nil, fmt.Errorf("label key and value must not be empty")
		}
		if len(key) > 64 || len(value) > 256 {
			return nil, fmt.Errorf("label %q exceeds length limit", key)
		}
		result[key] = value
	}
	return result, nil
}

func CloneLabels(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func WritableLabels(input map[string]string) map[string]string {
	return CloneLabels(input)
}

func SortedLabelKeys(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func LabelFingerprint(labels map[string]string) string {
	keys := SortedLabelKeys(labels)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, ";")
}
