package device

import (
	"fmt"
	"strconv"
	"strings"
)

type SemanticVersion struct {
	Major      int    `json:"major"`
	Minor      int    `json:"minor"`
	Patch      int    `json:"patch"`
	Prerelease string `json:"prerelease,omitempty"`
	Build      string `json:"build,omitempty"`
}

func ParseSemanticVersion(raw string) (SemanticVersion, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SemanticVersion{}, fmt.Errorf("version is required")
	}
	result := SemanticVersion{}
	coreAndBuild := strings.SplitN(raw, "+", 2)
	if len(coreAndBuild) == 2 {
		result.Build = coreAndBuild[1]
		if result.Build == "" {
			return SemanticVersion{}, fmt.Errorf("build metadata must not be empty")
		}
	}
	coreAndPrerelease := strings.SplitN(coreAndBuild[0], "-", 2)
	if len(coreAndPrerelease) == 2 {
		result.Prerelease = coreAndPrerelease[1]
		if result.Prerelease == "" {
			return SemanticVersion{}, fmt.Errorf("prerelease must not be empty")
		}
	}
	parts := strings.Split(coreAndPrerelease[0], ".")
	if len(parts) < 1 || len(parts) > 3 {
		return SemanticVersion{}, fmt.Errorf("version must have one to three numeric components")
	}
	values := make([]int, 3)
	for index, part := range parts {
		if part == "" {
			return SemanticVersion{}, fmt.Errorf("version component %d is empty", index+1)
		}
		if len(part) > 1 && part[0] == '0' {
			return SemanticVersion{}, fmt.Errorf("version component %d has a leading zero", index+1)
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return SemanticVersion{}, fmt.Errorf("version component %d is invalid", index+1)
		}
		values[index] = value
	}
	result.Major, result.Minor, result.Patch = values[0], values[1], values[2]
	return result, nil
}

func (v SemanticVersion) String() string {
	result := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		result += "-" + v.Prerelease
	}
	if v.Build != "" {
		result += "+" + v.Build
	}
	return result
}

func (v SemanticVersion) Compare(other SemanticVersion) int {
	if v.Major != other.Major {
		return compareInteger(v.Major, other.Major)
	}
	if v.Minor != other.Minor {
		return compareInteger(v.Minor, other.Minor)
	}
	if v.Patch != other.Patch {
		return compareInteger(v.Patch, other.Patch)
	}
	return comparePrerelease(v.Prerelease, other.Prerelease)
}

func compareInteger(left, right int) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func comparePrerelease(left, right string) int {
	if left == right {
		return 0
	}
	if left == "" {
		return 1
	}
	if right == "" {
		return -1
	}
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	maximum := len(leftParts)
	if len(rightParts) > maximum {
		maximum = len(rightParts)
	}
	for index := 0; index < maximum; index++ {
		if index >= len(leftParts) {
			return -1
		}
		if index >= len(rightParts) {
			return 1
		}
		leftNumber, leftErr := strconv.Atoi(leftParts[index])
		rightNumber, rightErr := strconv.Atoi(rightParts[index])
		switch {
		case leftErr == nil && rightErr == nil:
			if result := compareInteger(leftNumber, rightNumber); result != 0 {
				return result
			}
		case leftErr == nil:
			return -1
		case rightErr == nil:
			return 1
		default:
			if leftParts[index] < rightParts[index] {
				return -1
			}
			if leftParts[index] > rightParts[index] {
				return 1
			}
		}
	}
	return 0
}

type VersionConstraint struct {
	Minimum *SemanticVersion
	Maximum *SemanticVersion
}

func (c VersionConstraint) Match(version SemanticVersion) bool {
	if c.Minimum != nil && version.Compare(*c.Minimum) < 0 {
		return false
	}
	if c.Maximum != nil && version.Compare(*c.Maximum) > 0 {
		return false
	}
	return true
}

func ParseVersionConstraint(minimum, maximum string) (VersionConstraint, error) {
	constraint := VersionConstraint{}
	if minimum != "" {
		value, err := ParseSemanticVersion(minimum)
		if err != nil {
			return VersionConstraint{}, fmt.Errorf("parse minimum version: %w", err)
		}
		constraint.Minimum = &value
	}
	if maximum != "" {
		value, err := ParseSemanticVersion(maximum)
		if err != nil {
			return VersionConstraint{}, fmt.Errorf("parse maximum version: %w", err)
		}
		constraint.Maximum = &value
	}
	if constraint.Minimum != nil && constraint.Maximum != nil && constraint.Minimum.Compare(*constraint.Maximum) > 0 {
		return VersionConstraint{}, fmt.Errorf("minimum version exceeds maximum version")
	}
	return constraint, nil
}

func CompatibleSoftwareVersion(raw string, constraint VersionConstraint) (bool, error) {
	version, err := ParseSemanticVersion(raw)
	if err != nil {
		return false, err
	}
	return constraint.Match(version), nil
}
