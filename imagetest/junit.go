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
	"bytes"
	"fmt"

	"github.com/jstemmer/go-junit-report/v2/junit"
	"github.com/jstemmer/go-junit-report/v2/parser/gotest"
)

// converts `go test` outputs to a jUnit testSuite
func convertToTestSuite(results []string, classname string) junit.Testsuite {
	ts := junit.Testsuite{}
	for _, testResult := range results {
		tcs, err := convertToTestCase(testResult)
		if err != nil {
			continue
		}
		ts.Testcases = append(ts.Testcases, tcs...)
		for _, tc := range tcs {
			tc.Classname = classname

			ts.Tests++
			if tc.Skipped != nil {
				ts.Skipped++
			}
			if tc.Failure != nil {
				ts.Failures++
			}
		}
	}
	return ts
}

// converts a single `go test` output to jUnit TestCases
func convertToTestCase(in string) ([]junit.Testcase, error) {
	r := bytes.NewReader([]byte(in))
	report, err := gotest.NewParser().Parse(r)
	if err != nil {
		return nil, err
	}
	tss := junit.CreateFromReport(report, "")

	if len(tss.Suites) < 1 {
		return nil, fmt.Errorf("empty test suite")
	}
	return tss.Suites[0].Testcases, nil
}
