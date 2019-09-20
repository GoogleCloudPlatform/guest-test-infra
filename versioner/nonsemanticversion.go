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
	dateFormat     = "20060102"
)

// NonSemanticVer is for packages that follow nonsemantic version release
type NonSemanticVer struct {
	// in yyyyMMdd format
	date     string
	buildNum int
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
	_, err := strconv.Atoi(parts[0])
	if err != nil || len(parts[0]) != 8 {
		return nil, ErrInvalidNonSemanticVer
	}

	bn, err := strconv.Atoi(parts[1])
	if err != nil || bn < 0 {
		return nil, ErrInvalidCharacters
	}

	return &NonSemanticVer{parts[0], bn}, nil
}

// IncrementVersion increases takes the current version and
// returns the next release version
func (v NonSemanticVer) IncrementVersion() (NonSemanticVer, error) {
	today := time.Now().Format(dateFormat)
	if strings.Compare(today, v.date) == 0 {
		return NonSemanticVer{date: v.date, buildNum: v.buildNum + 1}, nil
	}
	return NonSemanticVer{date: today, buildNum: 0}, nil
}

// String returns the  in string format
func (v NonSemanticVer) String() string {
	return fmt.Sprintf("%s.%d", v.date, v.buildNum)
}

func (v NonSemanticVer) deepEquals(a NonSemanticVer) bool {
	return strings.Compare(v.date, a.date) == 0 && v.buildNum == a.buildNum
}
