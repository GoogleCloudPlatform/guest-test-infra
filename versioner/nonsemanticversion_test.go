package versioner

import (
	"strings"
	"testing"
	"time"
)

func TestNewNonSemanticVer(t *testing.T) {
	testCases := []struct {
		desc, nonSemanticVersion, expect string
		isErrorExpected                  bool
		expectedError                    string
	}{
		{"Happy case, pilot test", "20190515.23", "20190515.23", false, ""},
		{"Happy case, trim off build number leading 0", "12345678.05", "12345678.5", false, ""},
		{"Leading zeroes", "03221432.2", "03221432.2", false, ""},
		{"invalid version, less than 8 characters", "234232.3", "", true, ErrInvalidNonSemanticVer.Error()},
		{"invalid version, alphabets in version", "abcd1234.4", "", true, ErrInvalidNonSemanticVer.Error()},
		{"invalid version, more than 2 fields", "20190515.23.2", "", true, ErrInvalidNonSemanticVer.Error()},
		{"invalid version, special characters in fields", "abcd-123.32", "", true, ErrInvalidNonSemanticVer.Error()},
		{"invalid version, empty field", "12345678.", "", true, ErrInvalidCharacters.Error()},
		{"invalid version, does not have negative date", "-12345678.98", "", true, ErrInvalidNonSemanticVer.Error()},
		{"invalid version, does not have negatives build number", "12345678.-98", "", true, ErrInvalidCharacters.Error()},
		{"invalid version, empty string", "", "", true, ErrEmptyString.Error()},
	}

	for _, tc := range testCases {
		actual, err := NewNonSemanticVer(tc.nonSemanticVersion)
		if tc.isErrorExpected {
			if err == nil || strings.Compare(err.Error(), tc.expectedError) != 0 {
				t.Errorf("Desc:(%s): unexpected error type! expected(%s), got(%v)", tc.desc, tc.expectedError, err)
			}
			continue
		}

		if actual.String() != tc.expect {
			t.Errorf("Desc:(%s): test case failed! expected(%s), got(%s)", tc.desc, tc.expect, actual)
		}
	}
}

func TestNonSemanticVerIncrementVersion(t *testing.T) {
	dp := time.Now().AddDate(0, 0, -5).Format(dateFormat)
	dt := time.Now().Format(dateFormat)
	testCases := []struct {
		desc          string
		input, expect NonSemanticVer
	}{
		{"Happy case, first build today", NonSemanticVer{dp, 4}, NonSemanticVer{dt, 0}},
		{"Happy case, non first time build today", NonSemanticVer{dt, 4}, NonSemanticVer{dt, 5}},
	}

	for _, tc := range testCases {
		actual, err := tc.input.IncrementVersion()
		if err != nil {
			t.Fatalf("Unexpected error: +%v", err)
		}
		if !actual.deepEquals(tc.expect) {
			t.Errorf("Desc:(%s): test case failed! expected(%s), got(%s)", tc.desc, tc.expect, actual)
		}
	}
}
