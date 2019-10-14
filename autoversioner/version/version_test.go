//  Copyright 2019 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package version

import (
	"math/rand"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestNewVersion(t *testing.T) {
	testCases := []struct {
		desc, versionString, expect string
		isErrorExpected             bool
		expectedError               string
	}{
		{"Happy case, pilot test", "20190515.23", "20190515.23", false, ""},
		{"Happy case, dont trim off build number leading 0", "12345678.05", "12345678.05", false, ""},
		{"Happy case, Add Leading zeroes", "03221432.2", "03221432.02", false, ""},
		{"invalid versionString, less than 8 characters", "234232.3", "", true, ErrInvalidVersion.Error()},
		{"invalid versionString, alphabets in versionString", "abcd1234.4", "", true, ErrInvalidVersion.Error()},
		{"invalid versionString, more than 2 fields", "20190515.23.2", "", true, ErrInvalidVersion.Error()},
		{"invalid versionString, special characters in fields", "abcd-123.32", "", true, ErrInvalidVersion.Error()},
		{"invalid versionString, empty field", "12345678.", "", true, ErrInvalidCharacters.Error()},
		{"invalid versionString, does not have negative date", "-12345678.98", "", true, ErrInvalidVersion.Error()},
		{"invalid versionString, does not have negatives build number", "12345678.-98", "", true, ErrInvalidCharacters.Error()},
		{"invalid versionString, empty string", "", "", true, ErrEmptyString.Error()},
	}

	for _, tc := range testCases {
		actual, err := NewVersion(tc.versionString)
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

func TestIncrementVersion(t *testing.T) {
	dp := time.Now().AddDate(0, 0, -5).Format(DateFormat)
	dt := time.Now().Format(DateFormat)
	testCases := []struct {
		desc          string
		input, expect Version
	}{
		{"Happy case, first build today", Version{dp, 4}, Version{dt, 0}},
		{"Happy case, non first time build today", Version{dt, 4}, Version{dt, 5}},
		{"", Version{}, Version{dt, 0}},
	}

	for _, tc := range testCases {
		actual := tc.input.IncrementVersion()
		if !actual.deepEquals(tc.expect) {
			t.Errorf("Desc:(%s): test case failed! expected(%s), got(%s)", tc.desc, tc.expect, actual)
		}
	}
}

func TestVersionSorter(t *testing.T) {
	var versions []Version
	times := 1000
	for i := 0; i < times; i++ {
		d := randate()
		bn := randNum()
		versions = append(versions, Version{d, bn})
	}
	sort.Sort(Sorter(versions))
	for i := 1; i < times; i++ {
		if !(versions[i-1].IsLesser(versions[i]) || versions[i-1].deepEquals(versions[i])) {
			t.Errorf("sorting failed! (%+v) not less than (%+v)", versions[i-1], versions[i])
		}
	}
}

func randate() string {
	min := time.Date(2018, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
	max := time.Date(2019, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
	// keep the delta small enough to generate test cases with same build date
	delta := max - min

	sec := rand.Int63n(delta) + min
	return time.Unix(sec, 0).Format(DateFormat)
}

func randNum() int {
	min := 0
	max := 100
	delta := max - min

	return rand.Intn(delta) + min
}
