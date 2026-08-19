package application

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
	versionPattern    = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+){0,3}(?:[-+][A-Za-z0-9.-]+)?$`)
	modelPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
)

type Validator struct {
	errors []ValidationError
}

func NewValidator() *Validator {
	return &Validator{errors: make([]ValidationError, 0)}
}

func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.add(field, "is required")
	}
	return v
}

func (v *Validator) Length(field, value string, minimum, maximum int) *Validator {
	length := utf8.RuneCountInString(value)
	if length < minimum {
		v.add(field, fmt.Sprintf("must contain at least %d characters", minimum))
	}
	if maximum > 0 && length > maximum {
		v.add(field, fmt.Sprintf("must contain at most %d characters", maximum))
	}
	return v
}

func (v *Validator) Identifier(field, value string) *Validator {
	if value != "" && !identifierPattern.MatchString(value) {
		v.add(field, "contains unsupported characters")
	}
	return v
}

func (v *Validator) Version(field, value string) *Validator {
	if value != "" && !versionPattern.MatchString(value) {
		v.add(field, "must be a dotted numeric version with an optional suffix")
	}
	return v
}

func (v *Validator) HardwareModel(field, value string) *Validator {
	if value != "" && !modelPattern.MatchString(value) {
		v.add(field, "contains unsupported characters")
	}
	return v
}

func (v *Validator) URL(field, value string, allowHTTP bool) *Validator {
	if value == "" {
		return v
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		v.add(field, "must be an absolute URL")
		return v
	}
	if parsed.Scheme != "https" && !(allowHTTP && parsed.Scheme == "http") {
		v.add(field, "must use HTTPS")
	}
	if parsed.User != nil {
		v.add(field, "must not contain embedded credentials")
	}
	if parsed.Fragment != "" {
		v.add(field, "must not contain a fragment")
	}
	return v
}

func (v *Validator) Range(field string, value, minimum, maximum float64) *Validator {
	if value < minimum || value > maximum {
		v.add(field, fmt.Sprintf("must be between %s and %s", formatNumber(minimum), formatNumber(maximum)))
	}
	return v
}

func (v *Validator) Positive(field string, value int) *Validator {
	if value <= 0 {
		v.add(field, "must be greater than zero")
	}
	return v
}

func (v *Validator) NonNegative(field string, value int) *Validator {
	if value < 0 {
		v.add(field, "must not be negative")
	}
	return v
}

func (v *Validator) Future(field string, value *time.Time, now time.Time) *Validator {
	if value != nil && value.Before(now) {
		v.add(field, "must be in the future")
	}
	return v
}

func (v *Validator) OneOf(field, value string, allowed ...string) *Validator {
	if value == "" {
		return v
	}
	for _, candidate := range allowed {
		if value == candidate {
			return v
		}
	}
	v.add(field, "must be one of: "+strings.Join(allowed, ", "))
	return v
}

func (v *Validator) StringMap(field string, values map[string]string, maximum int) *Validator {
	if maximum > 0 && len(values) > maximum {
		v.add(field, fmt.Sprintf("must contain at most %d entries", maximum))
	}
	for key, value := range values {
		if strings.TrimSpace(key) == "" {
			v.add(field, "contains an empty key")
		}
		if strings.TrimSpace(value) == "" {
			v.add(field+"."+key, "must not be empty")
		}
		if utf8.RuneCountInString(key) > 64 {
			v.add(field, "contains a key longer than 64 characters")
		}
		if utf8.RuneCountInString(value) > 512 {
			v.add(field+"."+key, "must contain at most 512 characters")
		}
	}
	return v
}

func (v *Validator) UniqueStrings(field string, values []string) *Validator {
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			v.add(field+"["+strconv.Itoa(index)+"]", "must not be empty")
			continue
		}
		if _, ok := seen[value]; ok {
			v.add(field, fmt.Sprintf("contains duplicate value %q", value))
			continue
		}
		seen[value] = struct{}{}
	}
	return v
}

func (v *Validator) NoControlCharacters(field, value string) *Validator {
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			v.add(field, "contains control characters")
			break
		}
	}
	return v
}

func (v *Validator) MutuallyExclusive(firstField string, firstSet bool, secondField string, secondSet bool) *Validator {
	if firstSet && secondSet {
		v.add(firstField, "must not be provided together with "+secondField)
		v.add(secondField, "must not be provided together with "+firstField)
	}
	return v
}

func (v *Validator) AtLeastOne(fields ...FieldPresence) *Validator {
	for _, field := range fields {
		if field.Present {
			return v
		}
	}
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	v.add(strings.Join(names, ","), "at least one value is required")
	return v
}

type FieldPresence struct {
	Name    string
	Present bool
}

func (v *Validator) add(field, message string) {
	v.errors = append(v.errors, ValidationError{Field: field, Message: message})
}

func (v *Validator) Valid() bool {
	return len(v.errors) == 0
}

func (v *Validator) Errors() []ValidationError {
	result := append([]ValidationError(nil), v.errors...)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Field == result[j].Field {
			return result[i].Message < result[j].Message
		}
		return result[i].Field < result[j].Field
	})
	return result
}

func (v *Validator) Error() error {
	if len(v.errors) == 0 {
		return nil
	}
	return ValidationErrors(v.Errors())
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	parts := make([]string, 0, len(e))
	for _, item := range e {
		parts = append(parts, item.Error())
	}
	return strings.Join(parts, "; ")
}

func (e ValidationErrors) Fields() []string {
	seen := make(map[string]struct{}, len(e))
	result := make([]string, 0, len(e))
	for _, item := range e {
		if _, ok := seen[item.Field]; ok {
			continue
		}
		seen[item.Field] = struct{}{}
		result = append(result, item.Field)
	}
	sort.Strings(result)
	return result
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func ValidateDeviceCommand(command CreateDeviceCommand) error {
	return NewValidator().
		Required("name", command.Name).
		Length("name", command.Name, 1, 128).
		NoControlCharacters("name", command.Name).
		Required("hardware_model", command.HardwareModel).
		HardwareModel("hardware_model", command.HardwareModel).
		Length("hardware_model", command.HardwareModel, 1, 128).
		Required("software_version", command.SoftwareVersion).
		Version("software_version", command.SoftwareVersion).
		StringMap("labels", command.Labels, 64).
		Error()
}

func ValidateRolloutCommand(command CreateRolloutCommand, now time.Time) error {
	validator := NewValidator().
		Required("name", command.Name).
		Length("name", command.Name, 1, 160).
		Required("configuration_id", command.ConfigurationID).
		Identifier("configuration_id", command.ConfigurationID).
		OneOf("strategy", string(command.Strategy), "immediate", "scheduled", "percentage").
		NonNegative("batch_size", command.BatchSize).
		Range("batch_percent", float64(command.BatchPercent), 0, 100).
		UniqueStrings("device_ids", command.DeviceIDs).
		Future("scheduled_at", command.ScheduledAt, now).
		MutuallyExclusive("batch_size", command.BatchSize > 0, "batch_percent", command.BatchPercent > 0).
		AtLeastOne(FieldPresence{Name: "group_id", Present: command.GroupID != ""}, FieldPresence{Name: "device_ids", Present: len(command.DeviceIDs) > 0})
	if command.Strategy == "scheduled" && command.ScheduledAt == nil {
		validator.add("scheduled_at", "is required for scheduled strategy")
	}
	return validator.Error()
}

func ValidateSubscriptionCommand(command CreateSubscriptionCommand) error {
	return NewValidator().
		Required("name", command.Name).
		Length("name", command.Name, 1, 128).
		Required("url", command.URL).
		URL("url", command.URL, false).
		Required("secret", command.Secret).
		Length("secret", command.Secret, 16, 512).
		UniqueStrings("event_types", command.EventTypes).
		Error()
}
