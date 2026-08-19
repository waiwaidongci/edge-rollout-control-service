package configuration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var variablePattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func ContentDigest(content string) string {
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:])
}

func ValidateJSON(content string) error {
	var value any
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if value == nil {
		return fmt.Errorf("configuration must not be null")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return fmt.Errorf("unexpected additional configuration document")
	}
	return nil
}

func DecodeConfigurationDocument(content string) (any, error) {
	var value any
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return nil, fmt.Errorf("unexpected additional configuration document")
	}
	return value, nil
}

func ValidateSingleConfiguration(content string) error {
	decoder := json.NewDecoder(strings.NewReader(content))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return fmt.Errorf("unexpected additional configuration document")
	}
	return nil
}

func ExtractVariables(content string) []string {
	matches := variablePattern.FindAllStringSubmatch(content, -1)
	seen := make(map[string]struct{}, len(matches))
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		if _, ok := seen[match[1]]; ok {
			continue
		}
		seen[match[1]] = struct{}{}
		result = append(result, match[1])
	}
	sort.Strings(result)
	return result
}

func RenderVariables(content string, values map[string]string) (string, error) {
	missing := make([]string, 0)
	for _, name := range ExtractVariables(content) {
		if _, ok := values[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing configuration variables: %s", strings.Join(missing, ", "))
	}
	return variablePattern.ReplaceAllStringFunc(content, func(match string) string {
		name := variablePattern.FindStringSubmatch(match)[1]
		return values[name]
	}), nil
}

func Compatible(model string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, value := range allowed {
		if value == model || value == "*" {
			return true
		}
	}
	return false
}
