package versioner

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// SemanticVer represents a single semantic version.
type SemanticVer struct {
	major, minor, patch uint64
}

var (
	// ErrInvalidSemanticVer is returned a version is found to be invalid when
	// being parsed.
	ErrInvalidSemanticVer = fmt.Errorf("Invalid Semantic Version")

	// ErrEmptyString is returned when an empty string is passed in for parsing.
	ErrEmptyString = errors.New("Version string empty")

	// ErrInvalidCharacters is returned when invalid characters are found as
	// part of a version
	ErrInvalidCharacters = errors.New("Invalid characters in version")

)

// NewSemanticVer parses a given version and returns an instance of version
// or an error if unable to parse the version
func NewSemanticVer(v string) (*SemanticVer, error) {
	if len(v) == 0 {
		return nil, ErrEmptyString
	}

	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidSemanticVer
	}

	var intPart [3]int
	var err error
	for idx, part := range parts {
		intPart[idx], err = strconv.Atoi(part)
		if err != nil || intPart[idx] < 0 {
			return nil, ErrInvalidCharacters
		}
	}

	sv := &SemanticVer{
		major: uint64(intPart[0]),
		minor: uint64(intPart[1]),
		patch: uint64(intPart[2]),
	}

	return sv, nil
}

// String converts SemanticVersion object to a string. Note that if the
// original version contained a leading v, the semantic version will not
// contain the leading v
func (v SemanticVer) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

// IncrementPatch increments the patch version of a given version
func (v SemanticVer) IncrementPatch() SemanticVer {
	return SemanticVer{
		major: v.major,
		minor: v.minor,
		patch: v.patch + 1,
	}
}

// IncrementMinor increases the minor version of a given version
func (v SemanticVer) IncrementMinor() SemanticVer {
	return SemanticVer{
		major: v.major,
		minor: v.minor + 1,
		patch: 0,
	}
}

// IncrementMajor increases the major version of a given version
func (v SemanticVer) IncrementMajor() SemanticVer {
	return SemanticVer{
		major: v.major + 1,
		minor: 0,
		patch: 0,
	}
}

func (v SemanticVer) deepEquals(a SemanticVer) bool {
	return v.major == a.major && v.minor == a.minor && v.patch == a.patch
}