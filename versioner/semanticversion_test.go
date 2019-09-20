package versioner

import (
	"strings"
	"testing"
)

func TestNewSemanticVer(t *testing.T) {
  testCases := []struct {
		desc, semanticVersion, expect string
		isErrorExpected bool
		expectedError string
	}{
		{"Happy case, pilot test", "1.2.3", "1.2.3", false, ""},
		{"Happy case, 2 digit field", "12.32.5", "12.32.5", false, ""},
		{"Leading zeroes, should be trimmed off", "01.2.3", "1.2.3", false, ""},
		{"invalid version, less than 3 fields", "2.3", "", true, ErrInvalidSemanticVer.Error()},
		{"invalid version, alphabets in version", "a.b.c", "", true, ErrInvalidCharacters.Error()},
		{"invalid version, more than 3 fields", "1.2.3.4", "", true, ErrInvalidSemanticVer.Error()},
		{"invalid version, special characters in fields", "1-2.4.5", "", true, ErrInvalidCharacters.Error()},
		{"invalid version, empty field", "1.2.", "", true, ErrInvalidCharacters.Error()},
		{"invalid version, does not have negatives", "-1.2.3", "", true, ErrInvalidCharacters.Error()},
		{"invalid version, empty string", "", "", true, ErrEmptyString.Error()},
	}

  for _, tc := range testCases {
  	actual,err := NewSemanticVer(tc.semanticVersion)
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

func TestSemanticVer_IncrementMajor(t *testing.T) {
	testCases := []struct{
		desc string
		intput, expect SemanticVer
	}{
		{"normal case", SemanticVer{1,2,3}, SemanticVer{2, 0,0}},
		{"back to back major release", SemanticVer{2,0,0}, SemanticVer{3,0,0}},
		{"major release after a feature release", SemanticVer{1, 2, 0}, SemanticVer{2,  0, 0}},
	}

	for _, tc := range testCases {
		actual := tc.intput.IncrementMajor()
		if !tc.expect.deepEquals(actual) {
			t.Errorf("Desc:(%s) test cased failed! expected(%+v), got(%+v)", tc.desc, tc.expect, actual)
		}
	}
}

func TestSemanticVer_IncrementMinor(t *testing.T) {
	testCases := []struct{
		desc string
		intput, expect SemanticVer
	}{
		{"back to back feature release", SemanticVer{2,4,0}, SemanticVer{2,5,0}},
		{"minor release after a patch release", SemanticVer{1, 2, 123}, SemanticVer{1,  3, 0}},
	}

	for _, tc := range testCases {
		actual := tc.intput.IncrementMinor()
		if !tc.expect.deepEquals(actual) {
			t.Errorf("Desc:(%s) test cased failed! expected(%+v), got(%+v)", tc.desc, tc.expect, actual)
		}
	}
}

func TestSemanticVer_IncrementPatch(t *testing.T) {
	testCases := []struct{
		desc string
		intput, expect SemanticVer
	}{
		{"back to back patch release", SemanticVer{2,4,1}, SemanticVer{2,4, 2}},
		{"patch release after a major release", SemanticVer{12, 0, 0}, SemanticVer{12,  0, 1}},
	}

	for _, tc := range testCases {
		actual := tc.intput.IncrementPatch()
		if !tc.expect.deepEquals(actual) {
			t.Errorf("Desc:(%s) test cased failed! expected(%+v), got(%+v)", tc.desc, tc.expect, actual)
		}
	}
}