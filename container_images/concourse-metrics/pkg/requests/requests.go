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

package requests

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

// JobResultArgs defines the required and optional arguments for building a new
// job result request (in the form of a cloud monitoring time series request).
type JobResultArgs struct {
	EndTimestamp   *int64 // Optional - will default to now in ms.
	Job            string
	MetricPath     string
	Pipeline       string
	ProjectID      string
	ResultState    string
	StartTimestamp int64 // Epoch Milliseconds.
	Task           string
	Zone           string
}

func validateJobResultRequestInput(input *JobResultArgs) error {
	if strings.TrimSpace(input.ProjectID) == "" {
		return fmt.Errorf("empty project-id value")
	}
	if strings.TrimSpace(input.Zone) == "" {
		return fmt.Errorf("empty zone value")
	}
	if strings.TrimSpace(input.Pipeline) == "" {
		return fmt.Errorf("empty pipeline value")
	}
	if strings.TrimSpace(input.Job) == "" {
		return fmt.Errorf("empty job value")
	}
	if strings.TrimSpace(input.Task) == "" {
		return fmt.Errorf("empty task value")
	}
	if strings.TrimSpace(input.MetricPath) == "" {
		return fmt.Errorf("empty metric-path value")
	}
	state := strings.TrimSpace(input.ResultState)
	if state != "success" && state != "failure" {
		return fmt.Errorf("invalid state value")
	}

	// Don't let the end timestamp occur before the start timestamp.
	if input.EndTimestamp != nil && *input.EndTimestamp < input.StartTimestamp {
		return fmt.Errorf("end-timestamp cannot occur before start-timestamp")
	}

	return nil
}

// BuildJobResultRequest builds a new job result request object to submit to gcp cloud monitoring.
func BuildJobResultRequest(input JobResultArgs) (*monitoringpb.CreateTimeSeriesRequest, error) {
	// Provide a default for the endTimestamp if one was not provided.
	var endTimestamp int64
	if input.EndTimestamp == nil {
		endTimestamp = time.Now().UnixNano() / 1000000
	} else {
		endTimestamp = *input.EndTimestamp
	}

	if e := validateJobResultRequestInput(&input); e != nil {
		return nil, e
	}

	return &monitoringpb.CreateTimeSeriesRequest{
		Name: "projects/" + input.ProjectID,
		TimeSeries: []*monitoringpb.TimeSeries{{
			Metric: &metricpb.Metric{
				Type: "custom.googleapis.com/" + input.MetricPath,
				Labels: map[string]string{
					"result_state": input.ResultState,
				},
			},
			Resource: &monitoredres.MonitoredResource{
				Type: "generic_task",
				Labels: map[string]string{
					"project_id": input.ProjectID,
					"location":   input.Zone,
					"namespace":  input.Pipeline,
					"job":        input.Job,
					"task_id":    input.Task,
				},
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					EndTime: &timestamp.Timestamp{
						Seconds: endTimestamp / 1000,
					},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{
						// Int64 value here is the duration (measured in ms).
						Int64Value: endTimestamp - input.StartTimestamp,
					},
				},
			}},
		}},
	}, nil
}

// CoverageArgs defines the required and optional arguments for building a new
// coverage request (in the form of a cloud monitoring time series request).
type CoverageArgs struct {
	EndTimestamp    *int64 // Optional - will default to now in ms.
	MetricPath      string
	PackageName     string
	CoveragePercent int64
	ProjectID       string
}

// BuildCoverageRequest builds a new job result request object to submit to gcp cloud monitoring.
func BuildCoverageRequest(input CoverageArgs) (*monitoringpb.CreateTimeSeriesRequest, error) {
	// Provide a default for the endTimestamp if one was not provided.
	var endTimestamp int64
	if input.EndTimestamp == nil {
		endTimestamp = time.Now().UnixNano() / 1000000
	} else {
		endTimestamp = *input.EndTimestamp
	}

	return &monitoringpb.CreateTimeSeriesRequest{
		Name: "projects/" + input.ProjectID,
		TimeSeries: []*monitoringpb.TimeSeries{{
			Metric: &metricpb.Metric{
				Type: "custom.googleapis.com/" + input.MetricPath,
			},
			Resource: &monitoredres.MonitoredResource{
				Type: "generic_task",
				Labels: map[string]string{
					"package_name": input.PackageName,
				},
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					EndTime: &timestamp.Timestamp{
						Seconds: endTimestamp / 1000,
					},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{
						// Int64 value here is the coverage (in percent of statements).
						Int64Value: input.CoveragePercent,
					},
				},
			}},
		}},
	}, nil
}
