package versioner

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrInvalidNonSemanticVer is returned if a version is found to be invalid when
	// being parsed.
	ErrInvalidNonSemanticVer = fmt.Errorf("Invalid NonSemantic Version")

	// ErrInvalidDate is returned if the date being parsed is in a different
	// format than expected.
	ErrInvalidDate = fmt.Errorf("Invalid date format")
	dateFormat     = "01-02-2006"
)

const (
	allowedDigits = "0123456789"
)

// NonSemanticVer is for packages that follow nonsemantic version release
type NonSemanticVer struct {
	// in yyyyMMdd format
	date     string
	buildNum uint64
}

// NewNonSemanticVer returns a new non semantic version object
func NewNonSemanticVer(v string) (*NonSemanticVer, error) {
	if len(v) == 0 {
		return nil, ErrEmptyString
	}

	parts := strings.Split(v, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidNonSemanticVer
	}

	// we do not check if it is a valid date in yyyyMMdd format
	// because we never use it for any computation
	if len(parts[0]) != 8 || !containsOnly(parts[0], allowedDigits) {
		return nil, ErrInvalidNonSemanticVer
	}

	bn, err := strconv.Atoi(parts[1])
	if err != nil || bn < 0 {
		return nil, ErrInvalidCharacters
	}

	return &NonSemanticVer{parts[0], uint64(bn)}, nil
}

// IncrementVersion increases takes the current version and
// returns the next release version
func (v NonSemanticVer) IncrementVersion() (*NonSemanticVer, error) {
	today := time.Now().Format(dateFormat)
	today, err := GetDateInBuildFormat(today)
	if err != nil {
		return nil, err
	}
	if strings.Compare(today, v.date) == 0 {
		return &NonSemanticVer{date: v.date, buildNum: v.buildNum + 1}, nil
	}
	return &NonSemanticVer{date: today, buildNum: uint64(0)}, nil
}

// String returns the  in string format
func (v NonSemanticVer) String() string {
	return fmt.Sprintf("%s.%d", v.date, v.buildNum)
}

func (v NonSemanticVer) deepEquals(a NonSemanticVer) bool {
	return strings.Compare(v.date, a.date) == 0 && v.buildNum == a.buildNum
}

// containsOnly makes sure that it has only valid characters.
func containsOnly(s string, comp string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return !strings.ContainsRune(comp, r)
	}) == -1
}

// GetDateInBuildFormat :
// the nonsematicversion format is yyyyMMdd.[\d+]. since golang
// does not have native implementation to convert date in this format,
// we provide this small utility to convert date to this format.
// This function only accepts MM-dd-yyyy format
// Sample usage:
//
// today := GetDateInBuildFormat(time.Now().Format("10-22-2019"))
//
//
func GetDateInBuildFormat(v string) (string, error) {
	if len(v) == 0 {
		return "", ErrEmptyString
	}

	parts := strings.Split(v, "-")
	if len(parts) != 3 || len(parts[0]) != 2 || len(parts[1]) != 2 || len(parts[2]) != 4 {
		return "", ErrInvalidDate
	}
	// convert to yyyyMMdd
	return fmt.Sprintf("%s%s%s", parts[2], parts[0], parts[1]), nil
}
