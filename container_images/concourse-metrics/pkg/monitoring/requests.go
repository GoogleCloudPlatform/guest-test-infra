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

package monitoring

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

type JobResultArgs struct {
	EndTimestamp   *int64
	Job            string
	MetricPath     string
	Pipeline       string
	ProjectId      string
	ResultState    string
	StartTimestamp int64
	Task           string
	Zone           string
}

func validateJobResultRequestInput(input *JobResultArgs) error {
	if strings.TrimSpace(input.ProjectId) == "" {
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

func BuildJobResultRequest(input JobResultArgs) (*monitoringpb.CreateTimeSeriesRequest, error) {
	// Provide a default for the endTimestamp if one was not provided.
	if input.EndTimestamp == nil {
		*input.EndTimestamp = time.Now().UnixNano() / 1000000
	}

	e := validateJobResultRequestInput(&input)
	if e != nil {
		return nil, e
	}

	// Calculate the duration to publish.
	duration := *input.EndTimestamp - input.StartTimestamp

	return &monitoringpb.CreateTimeSeriesRequest{
		Name: "projects/" + input.ProjectId,
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
					"project_id": input.ProjectId,
					"location":   input.Zone,
					"namespace":  input.Pipeline,
					"job":        input.Job,
					"task_id":    input.Task,
				},
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					EndTime: &timestamp.Timestamp{
						Seconds: *input.EndTimestamp / 1000,
					},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{
						Int64Value: duration,
					},
				},
			}},
		}},
	}, nil
}
