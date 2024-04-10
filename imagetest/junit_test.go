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
	"github.com/jstemmer/go-junit-report/v2/junit"
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
    main_test.go:47: failed, message: heh
    main_test.go:47: failed, message: heh2
    main_test.go:47: failed, message: heh again
--- FAIL: TestAlwaysFails (0.00s)
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
		ts      junit.Testsuite
	}{
		{
			[]string{testPass},
			junit.Testsuite{Tests: 4},
		},
		{
			[]string{testFail},
			junit.Testsuite{Tests: 5, Failures: 1},
		},
		{
			[]string{testPass, testPass},
			junit.Testsuite{Tests: 8},
		},
		{
			[]string{testPass, testFail},
			junit.Testsuite{Tests: 9, Failures: 1},
		},
	}
	for idx, tt := range tests {
		ts := convertToTestSuite(tt.results, "")
		switch {
		case ts.Name != tt.ts.Name:
			t.Errorf("test %d Name got: %+v, want: %+v", idx, ts.Name, tt.ts.Name)
		case ts.Tests != tt.ts.Tests:
			t.Errorf("test %d Tests got: %+v, want: %+v", idx, ts.Tests, tt.ts.Tests)
		case ts.Failures != tt.ts.Failures:
			t.Errorf("test %d Failures got: %+v, want: %+v", idx, ts.Failures, tt.ts.Failures)
		case ts.Errors != tt.ts.Errors:
			t.Errorf("test %d Errors got: %+v, want: %+v", idx, ts.Errors, tt.ts.Errors)
		case ts.Disabled != tt.ts.Disabled:
			t.Errorf("test %d Disabled got: %+v, want: %+v", idx, ts.Disabled, tt.ts.Disabled)
		case ts.Skipped != tt.ts.Skipped:
			t.Errorf("test %d Skipped got: %+v, want: %+v", idx, ts.Skipped, tt.ts.Skipped)
		case ts.Time != tt.ts.Time:
			t.Errorf("test %d Time got: %+v, want: %+v", idx, ts.Time, tt.ts.Time)
		case ts.SystemOut != tt.ts.SystemOut:
			t.Errorf("test %d SystemOut got: %+v, want: %+v", idx, ts.SystemOut, tt.ts.SystemOut)
		case ts.SystemErr != tt.ts.SystemErr:
			t.Errorf("test %d SystemErr got: %+v, want: %+v", idx, ts.SystemErr, tt.ts.SystemErr)
		case len(ts.Testcases) != tt.ts.Tests:
			t.Errorf("test %d test length got: %+v, want: %+v", idx, ts.Tests, tt.ts.Tests)
		}
	}
}

func TestConvertToTestCase(t *testing.T) {
	tests := []struct {
		result string
		tcs    []junit.Testcase
	}{
		{
			testPass,
			[]junit.Testcase{
				{Name: "TestUpdateNSSwitchConfig", Time: "0.000"},
				{Name: "TestUpdateSSHConfig", Time: "0.000"},
				{Name: "TestUpdatePAMsshd", Time: "0.000"},
				{Name: "TestUpdateGroupConf", Time: "0.000"}},
		},
		{
			testFail,
			[]junit.Testcase{
				{Time: "0.000", Name: "TestAlwaysFails", Failure: &junit.Result{
					Data: "    main_test.go:47: failed, message: heh\n    main_test.go:47: failed, message: heh2\n    main_test.go:47: failed, message: heh again"},
				},
			{Time: "0.000", Name: "TestUpdateNSSwitchConfig"},
				{Time: "0.000", Name: "TestUpdateSSHConfig"},
				{Time: "0.000", Name: "TestUpdatePAMsshd"},
				{Time: "0.000", Name: "TestUpdateGroupConf"}},
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
				t.Errorf("test %d mismatched Classname in test case %d. got: %v but want: %v", idx, i, tcs[i].Classname, tt.tcs[i].Classname)
			case tcs[i].Name != tt.tcs[i].Name:
				t.Errorf("test %d mismatched Name in test case %d. got: %v but want: %v", idx, i, tcs[i].Name, tt.tcs[i].Name)
			case tcs[i].Time != tt.tcs[i].Time:
				t.Errorf("test %d mismatched Time test case %d. got: %v but want: %v", idx, i, tcs[i].Time, tt.tcs[i].Time)
			case tcs[i].Skipped != tt.tcs[i].Skipped:
				t.Errorf("test %d mismatched Skipped in test case %d. got: %v but want: %v", idx, i, tcs[i].Skipped, tt.tcs[i].Skipped)
			case (tcs[i].Failure != nil && tt.tcs[i].Failure == nil) || (tcs[i].Failure == nil && tt.tcs[i].Failure != nil) :
				t.Errorf("test %d mismatched Failure status in test case %d. got: %v but want: %v", idx, i, tcs[i].Failure, tt.tcs[i].Failure)
			case tcs[i].Failure != nil && tcs[i].Failure.Data != tt.tcs[i].Failure.Data :
				t.Errorf("test %d mismatched Failure Data in test case %d. got: %v but want: %v", idx, i, tcs[i].Failure.Data, tt.tcs[i].Failure.Data)
			case tcs[i].SystemOut != tt.tcs[i].SystemOut:
				t.Errorf("test %d mismatched SystemOut in test case %d. got: %v but want: %v", idx, i, tcs[i].SystemOut, tt.tcs[i].SystemOut)
			}
		}
	}
}
