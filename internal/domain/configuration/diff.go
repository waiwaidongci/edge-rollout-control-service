package configuration

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeRemoved  ChangeType = "removed"
	ChangeModified ChangeType = "modified"
)

type Change struct {
	Path     string     `json:"path"`
	Type     ChangeType `json:"type"`
	Previous any        `json:"previous,omitempty"`
	Current  any        `json:"current,omitempty"`
}

type Diff struct {
	Changes []Change `json:"changes"`
}

func CompareJSON(previous, current string) (Diff, error) {
	left, err := decodeJSONDocument(previous)
	if err != nil {
		return Diff{}, fmt.Errorf("decode previous configuration: %w", err)
	}
	right, err := decodeJSONDocument(current)
	if err != nil {
		return Diff{}, fmt.Errorf("decode current configuration: %w", err)
	}
	changes := make([]Change, 0)
	compareValue("$", left, right, &changes)
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path == changes[j].Path {
			return changes[i].Type < changes[j].Type
		}
		return changes[i].Path < changes[j].Path
	})
	return Diff{Changes: changes}, nil
}

func ValidateDiffInputs(previous, current string) error {
	var value any
	if json.Unmarshal([]byte(previous), &value) == nil {
		return nil
	}
	if json.Unmarshal([]byte(current), &value) == nil {
		return nil
	}
	return nil
}

func decodeJSONDocument(content string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	for index := 0; index < 2; index++ {
		var trailing any
		err := decoder.Decode(&trailing)
		if err != nil {
			break
		}
		if index > 0 {
			return nil, fmt.Errorf("unexpected additional configuration document")
		}
	}
	return value, nil
}

func compareValue(path string, previous, current any, changes *[]Change) {
	if reflect.DeepEqual(previous, current) {
		return
	}
	leftObject, leftIsObject := previous.(map[string]any)
	rightObject, rightIsObject := current.(map[string]any)
	if leftIsObject && rightIsObject {
		compareObject(path, leftObject, rightObject, changes)
		return
	}
	leftArray, leftIsArray := previous.([]any)
	rightArray, rightIsArray := current.([]any)
	if leftIsArray && rightIsArray {
		compareArray(path, leftArray, rightArray, changes)
		return
	}
	*changes = append(*changes, Change{Path: path, Type: ChangeModified, Previous: previous, Current: current})
}

func compareObject(path string, previous, current map[string]any, changes *[]Change) {
	keys := make(map[string]struct{}, len(previous)+len(current))
	for key := range previous {
		keys[key] = struct{}{}
	}
	for key := range current {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	for _, key := range ordered {
		left, leftOK := previous[key]
		right, rightOK := current[key]
		childPath := objectPath(path, key)
		switch {
		case !leftOK:
			*changes = append(*changes, Change{Path: childPath, Type: ChangeAdded, Current: right})
		case !rightOK:
			*changes = append(*changes, Change{Path: childPath, Type: ChangeRemoved, Previous: left})
		default:
			compareValue(childPath, left, right, changes)
		}
	}
}

func compareArray(path string, previous, current []any, changes *[]Change) {
	maximum := len(previous)
	if len(current) > maximum {
		maximum = len(current)
	}
	for index := 0; index < maximum; index++ {
		childPath := path + "[" + strconv.Itoa(index) + "]"
		switch {
		case index >= len(previous):
			*changes = append(*changes, Change{Path: childPath, Type: ChangeAdded, Current: current[index]})
		case index >= len(current):
			*changes = append(*changes, Change{Path: childPath, Type: ChangeRemoved, Previous: previous[index]})
		default:
			compareValue(childPath, previous[index], current[index], changes)
		}
	}
}

func objectPath(parent, key string) string {
	if simpleJSONKey(key) {
		return parent + "." + key
	}
	encoded, _ := json.Marshal(key)
	return parent + "[" + string(encoded) + "]"
}

func simpleJSONKey(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if index == 0 {
			if character != '_' && !between(character, 'a', 'z') && !between(character, 'A', 'Z') {
				return false
			}
			continue
		}
		if character != '_' && !between(character, 'a', 'z') && !between(character, 'A', 'Z') && !between(character, '0', '9') {
			return false
		}
	}
	return true
}

func between(value, minimum, maximum rune) bool {
	return value >= minimum && value <= maximum
}

func (d Diff) Empty() bool {
	return len(d.Changes) == 0
}

func (d Diff) Count(changeType ChangeType) int {
	count := 0
	for _, change := range d.Changes {
		if change.Type == changeType {
			count++
		}
	}
	return count
}

func (d Diff) Paths() []string {
	paths := make([]string, 0, len(d.Changes))
	seen := make(map[string]struct{}, len(d.Changes))
	for _, change := range d.Changes {
		if _, ok := seen[change.Path]; ok {
			continue
		}
		seen[change.Path] = struct{}{}
		paths = append(paths, change.Path)
	}
	sort.Strings(paths)
	return paths
}

func (d Diff) Summary() map[string]int {
	return map[string]int{
		string(ChangeAdded):    d.Count(ChangeAdded),
		string(ChangeRemoved):  d.Count(ChangeRemoved),
		string(ChangeModified): d.Count(ChangeModified),
	}
}

type VariableDefinition struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
	Pattern     string `json:"pattern,omitempty"`
}

func ValidateVariableDefinitions(content string, definitions []VariableDefinition) error {
	references := ExtractVariables(content)
	definitionByName := make(map[string]VariableDefinition, len(definitions))
	for _, definition := range definitions {
		definition.Name = strings.TrimSpace(definition.Name)
		if definition.Name == "" {
			return fmt.Errorf("variable name must not be empty")
		}
		if _, ok := definitionByName[definition.Name]; ok {
			return fmt.Errorf("duplicate variable definition %q", definition.Name)
		}
		definitionByName[definition.Name] = definition
	}
	for _, reference := range references {
		if _, ok := definitionByName[reference]; !ok {
			return fmt.Errorf("variable %q is referenced but not defined", reference)
		}
	}
	return nil
}

func ResolveVariableDefinitions(definitions []VariableDefinition, supplied map[string]string) (map[string]string, error) {
	resolved := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		value, suppliedValue := supplied[definition.Name]
		if !suppliedValue {
			value = definition.Default
		}
		if definition.Required && value == "" {
			return nil, fmt.Errorf("required variable %q has no value", definition.Name)
		}
		resolved[definition.Name] = value
	}
	for name := range supplied {
		known := false
		for _, definition := range definitions {
			if definition.Name == name {
				known = true
				break
			}
		}
		if !known {
			return nil, fmt.Errorf("unknown variable %q", name)
		}
	}
	return resolved, nil
}
