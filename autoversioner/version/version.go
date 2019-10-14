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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrInvalidVersion is returned if a versionString is found to be invalid when
	// being parsed.
	ErrInvalidVersion = errors.New("Invalid Version")

	// ErrInvalidDate is returned if the date being parsed is in a different
	// format than expected.
	ErrInvalidDate = errors.New("Invalid date format")

	// ErrEmptyString is returned when an empty string is passed in for parsing.
	ErrEmptyString = errors.New("Version string empty")

	// ErrInvalidCharacters is returned when invalid characters are found as
	// part of a versionString
	ErrInvalidCharacters = errors.New("Invalid characters in versionString")
)

const (
	// DateFormat is format of tag we chose to use in github
	DateFormat = "20060102"
)

// Version is version of a package released
type Version struct {
	// in yyyyMMdd format
	date     string
	buildNum int
}

// Sorter sorts a list of Version
type Sorter []Version

func (a Sorter) Len() int {
	return len(a)
}

func (a Sorter) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// sort in increasing order
func (a Sorter) Less(i, j int) bool {
	if a[i].date == a[j].date {
		return a[i].buildNum < a[j].buildNum
	}
	ti, _ := time.Parse(DateFormat, a[i].date)
	tj, _ := time.Parse(DateFormat, a[j].date)

	return tj.After(ti)
}

// NewVersion returns a new non semantic versionString object
func NewVersion(v string) (*Version, error) {
	if len(v) == 0 {
		return nil, ErrEmptyString
	}

	parts := strings.Split(v, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidVersion
	}

	// we do not check if it is a valid date in yyyyMMdd format
	// because we never use it for any computation
	_, err := strconv.Atoi(parts[0])
	if err != nil || len(parts[0]) != 8 {
		return nil, ErrInvalidVersion
	}

	bn, err := strconv.Atoi(parts[1])
	if err != nil || bn < 0 {
		return nil, ErrInvalidCharacters
	}

	return &Version{parts[0], bn}, nil
}

// IncrementVersion increases takes the current versionString and
// returns the next release versionString
func (v Version) IncrementVersion() Version {
	today := time.Now().Format(DateFormat)
	if strings.Compare(today, v.date) == 0 {
		return Version{date: v.date, buildNum: v.buildNum + 1}
	}
	return Version{date: today, buildNum: 0}
}

// String returns the  in string format
func (v Version) String() string {
	return fmt.Sprintf("%s.%02d", v.date, v.buildNum)
}

func (v Version) deepEquals(a Version) bool {
	return strings.Compare(v.date, a.date) == 0 && v.buildNum == a.buildNum
}

// IsLesser checks if a versionString was generated earlier than a given versionString
func (v Version) IsLesser(a Version) bool {
	if v.date == a.date {
		return v.buildNum < a.buildNum
	}
	tv, _ := time.Parse(DateFormat, v.date)
	ta, _ := time.Parse(DateFormat, a.date)
	return ta.After(tv)
}
