// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package imagetest

import (
	"testing"
)

var (
	testPass = `
=== RUN   TestUpdateNSSwitchConfig
--- PASS: TestUpdateNSSwitchConfig (0.00s)
=== RUN   TestUpdateSSHConfig
--- PASS: TestUpdateSSHConfig (0.00s)
=== RUN   TestUpdatePAMsshd
--- PASS: TestUpdatePAMsshd (0.00s)
=== RUN   TestUpdateGroupConf
--- PASS: TestUpdateGroupConf (0.00s)
PASS
`
	testFail = `
=== RUN   TestAlwaysFails
--- FAIL: TestAlwaysFails (0.00s)
    main_test.go:47: failed, message: heh
    main_test.go:47: failed, message: heh2
    main_test.go:47: failed, message: heh again
=== RUN   TestUpdateNSSwitchConfig
--- PASS: TestUpdateNSSwitchConfig (0.00s)
=== RUN   TestUpdateSSHConfig
--- PASS: TestUpdateSSHConfig (0.00s)
=== RUN   TestUpdatePAMsshd
--- PASS: TestUpdatePAMsshd (0.00s)
=== RUN   TestUpdateGroupConf
--- PASS: TestUpdateGroupConf (0.00s)
FAIL
`
)

func TestConvertToTestSuite(t *testing.T) {
	tests := []struct {
		results []string
		ts      *testSuite
	}{
		{
			[]string{testPass},
			&testSuite{Tests: 4},
		},
		{
			[]string{testFail},
			&testSuite{Tests: 5, Failures: 1},
		},
		{
			[]string{testPass, testPass},
			&testSuite{Tests: 8},
		},
		{
			[]string{testPass, testFail},
			&testSuite{Tests: 9, Failures: 1},
		},
	}
	for idx, tt := range tests {
		ts := convertToTestSuite(tt.results)
		switch {
		case ts.XMLName != tt.ts.XMLName:
			fallthrough
		case ts.Name != tt.ts.Name:
			fallthrough
		case ts.Tests != tt.ts.Tests:
			fallthrough
		case ts.Failures != tt.ts.Failures:
			fallthrough
		case ts.Errors != tt.ts.Errors:
			fallthrough
		case ts.Disabled != tt.ts.Disabled:
			fallthrough
		case ts.Skipped != tt.ts.Skipped:
			fallthrough
		case ts.Time != tt.ts.Time:
			fallthrough
		case ts.SystemOut != tt.ts.SystemOut:
			fallthrough
		case ts.SystemErr != tt.ts.SystemErr:
			fallthrough
		case len(ts.TestCase) != tt.ts.Tests:
			t.Errorf("test %d expected: %+v got: %+v", idx, tt.ts, ts)
		}
	}
}

func TestConvertToTestCase(t *testing.T) {
	tests := []struct {
		result string
		tcs    []*testCase
	}{
		{
			testPass,
			[]*testCase{
				{Name: "TestUpdateNSSwitchConfig"},
				{Name: "TestUpdateSSHConfig"},
				{Name: "TestUpdatePAMsshd"},
				{Name: "TestUpdateGroupConf"}},
		},
		{
			testFail,
			[]*testCase{
				{Name: "TestAlwaysFails", Failure: &junitFailure{
					FailMessage: "main_test.go:47: failed, message: heh\nmain_test.go:47: failed, message: heh2\nmain_test.go:47: failed, message: heh again"},
				},
				{Name: "TestUpdateNSSwitchConfig"},
				{Name: "TestUpdateSSHConfig"},
				{Name: "TestUpdatePAMsshd"},
				{Name: "TestUpdateGroupConf"}},
		},
	}

	for idx, tt := range tests {
		tcs, err := convertToTestCase(tt.result)
		if err != nil {
			t.Errorf("test %d error parsing: %v", idx, err)
			continue
		}
		if len(tcs) != len(tt.tcs) {
			t.Errorf("test %d expected: %v got: %v", idx, tt.tcs, tcs)
			continue
		}
		for i := 0; i < len(tt.tcs); i++ {
			switch {
			case tcs[i].Classname != tt.tcs[i].Classname:
				fallthrough
			case tcs[i].Name != tt.tcs[i].Name:
				fallthrough
			case tcs[i].Time != tt.tcs[i].Time:
				fallthrough
			case tcs[i].Skipped != tt.tcs[i].Skipped:
				fallthrough
			case tcs[i].Failure != nil && tcs[i].Failure.FailMessage != tt.tcs[i].Failure.FailMessage:
				fallthrough
			case tcs[i].SystemOut != tt.tcs[i].SystemOut:
				t.Errorf("test %d mismatched test case %d. expected: %v got: %v", idx, i, tt.tcs[i], tcs[i])
			}
		}
	}
}
