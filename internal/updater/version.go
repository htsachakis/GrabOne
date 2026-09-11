// Package updater checks GitHub for a newer GrabOne release, downloads the
// installer and verifies it before it is ever run.
//
// The application never installs an update on its own: it reports what is
// available, and the user decides.
package updater

import (
	"strconv"
	"strings"
)

// NormalizeVersion strips the leading "v" and surrounding space from a tag.
func NormalizeVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	return strings.TrimPrefix(strings.TrimPrefix(trimmed, "v"), "V")
}

// CompareVersions orders two versions, returning -1, 0 or 1.
//
// Versions are compared component by component as numbers, so 1.10.0 is newer
// than 1.9.0. A pre-release suffix marks a version as older than the same
// version without one, which is what keeps a released 1.2.0 ahead of 1.2.0-rc1.
func CompareVersions(left, right string) int {
	leftCore, leftPre := splitVersion(NormalizeVersion(left))
	rightCore, rightPre := splitVersion(NormalizeVersion(right))

	leftParts := numericParts(leftCore)
	rightParts := numericParts(rightCore)

	length := max(len(leftParts), len(rightParts))
	for index := 0; index < length; index++ {
		leftValue, rightValue := partAt(leftParts, index), partAt(rightParts, index)
		if leftValue != rightValue {
			if leftValue < rightValue {
				return -1
			}
			return 1
		}
	}

	switch {
	case leftPre == rightPre:
		return 0
	case leftPre == "":
		// A release is newer than any pre-release of the same version.
		return 1
	case rightPre == "":
		return -1
	case leftPre < rightPre:
		return -1
	default:
		return 1
	}
}

// IsNewer reports whether candidate is a later version than current.
func IsNewer(candidate, current string) bool {
	return CompareVersions(candidate, current) > 0
}

// splitVersion separates "1.2.3-rc1+build" into its core and pre-release parts.
func splitVersion(version string) (string, string) {
	if index := strings.IndexByte(version, '+'); index >= 0 {
		version = version[:index]
	}
	if index := strings.IndexByte(version, '-'); index >= 0 {
		return version[:index], version[index+1:]
	}
	return version, ""
}

// numericParts reads the dotted numbers of a version, ignoring anything that is
// not a number so an unexpected tag cannot break the comparison.
func numericParts(core string) []int {
	if core == "" {
		return nil
	}

	fields := strings.Split(core, ".")
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		value, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil {
			value = 0
		}
		parts = append(parts, value)
	}
	return parts
}

func partAt(parts []int, index int) int {
	if index < len(parts) {
		return parts[index]
	}
	return 0
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
