package imagetest

import (
	"bytes"
	"encoding/xml"
	"fmt"

	junitFormatter "github.com/jstemmer/go-junit-report/formatter"
	junitParser "github.com/jstemmer/go-junit-report/parser"
)

type testSuites struct {
	XMLName   xml.Name     `xml:"testsuites"`
	Name      string       `xml:"name,attr"`
	Errors    int          `xml:"errors,attr"`
	Failures  int          `xml:"failures,attr"`
	Tests     int          `xml:"tests,attr"`
	Time      float64      `xml:"time,attr"`
	TestSuite []*testSuite `xml:"testsuite"`
}

type testSuite struct {
	XMLName   xml.Name `xml:"testsuite"`
	Name      string   `xml:"name,attr"`
	Tests     int      `xml:"tests,attr"`
	Failures  int      `xml:"failures,attr"`
	Errors    int      `xml:"errors,attr"`
	Disabled  int      `xml:"disabled,attr"`
	Skipped   int      `xml:"skipped,attr"`
	Time      float64  `xml:"time,attr"`
	SystemOut string   `xml:"system-out,omitempty"`
	SystemErr string   `xml:"system-err,omitempty"`

	TestCase []*testCase `xml:"testcase"`
}

type testCase struct {
	Classname string        `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	Time      float64       `xml:"time,attr"`
	Skipped   *junitSkipped `xml:"skipped,omitempty"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	SystemOut string        `xml:"system-out,omitempty"`
}

type junitSkipped struct {
	Message string `xml:"message,attr"`
}

type junitFailure struct {
	FailMessage string `xml:",chardata"`
	FailType    string `xml:"type,attr"`
}

// converts `go test` outputs to a jUnit testSuite
func convertToTestSuite(results []string) *testSuite {
	ts := &testSuite{}
	for _, testResult := range results {
		tcs, err := convertToTestCase(testResult)
		if err != nil {
			continue
		}
		ts.TestCase = append(ts.TestCase, tcs...)
		for _, tc := range tcs {
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
func convertToTestCase(in string) ([]*testCase, error) {
	var b bytes.Buffer
	r := bytes.NewReader([]byte(in))
	report, err := junitParser.Parse(r, "")
	if err != nil {
		return nil, err
	}
	if err = junitFormatter.JUnitReportXML(report, false, "", &b); err != nil {
		return nil, err
	}

	var tss testSuites
	if err := xml.Unmarshal(b.Bytes(), &tss); err != nil {
		return nil, err
	}

	if len(tss.TestSuite) < 1 || tss.TestSuite[0] == nil || tss.TestSuite[0].TestCase == nil {
		return nil, fmt.Errorf("empty test suite")
	}
	return tss.TestSuite[0].TestCase, nil
}
