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
	"reflect"
	"strings"
	"testing"
)

const (
	job            = "test-job"
	metricPath     = "metric-path"
	pipeline       = "test-pipeline"
	projectID      = "test-project"
	resultState    = "success"
	startTimestamp = 1620363558000
	task           = "test-task"
	zone           = "test-zone"
)

func TestBuildJobResultRequest(t *testing.T) {
	// Instantiated since you can't make pointers to int literals (since they are not addressable).
	validEndTimestamp := int64(1620363568000)

	input := JobResultArgs{
		EndTimestamp:   &validEndTimestamp,
		Job:            job,
		MetricPath:     metricPath,
		Pipeline:       pipeline,
		ProjectID:      projectID,
		ResultState:    resultState,
		StartTimestamp: startTimestamp,
		Task:           task,
		Zone:           zone,
	}
	result, err := BuildJobResultRequest(input)
	if err != nil {
		t.Errorf("Happy path BuildJobResultRequest should not return an error: %+v", err)
	}

	assertEqual(t, result.Name, "projects/"+projectID)
	assertEqual(t, result.TimeSeries[0].Metric.Type, "custom.googleapis.com/"+metricPath)
	assertEqual(t, result.TimeSeries[0].Metric.Labels["result_state"], resultState)
	assertEqual(t, result.TimeSeries[0].Resource.Type, "generic_task")
	assertEqual(t, result.TimeSeries[0].Resource.Labels["project_id"], projectID)
	assertEqual(t, result.TimeSeries[0].Resource.Labels["location"], zone)
	assertEqual(t, result.TimeSeries[0].Resource.Labels["namespace"], pipeline)
	assertEqual(t, result.TimeSeries[0].Resource.Labels["job"], job)
	assertEqual(t, result.TimeSeries[0].Resource.Labels["task_id"], task)
	assertEqual(t, result.TimeSeries[0].Points[0].Interval.EndTime.Seconds, validEndTimestamp/1000)
	assertEqual(t, result.TimeSeries[0].Points[0].Value.GetInt64Value(), validEndTimestamp-startTimestamp)
}

func TestBuildJobResultRequestInputValidation(t *testing.T) {
	// Instantiated since you can't make pointers to int literals (since they are not addressable).
	validEndTimestamp := int64(1620363568000)

	tests := []struct {
		args    JobResultArgs
		message string
	}{
		{JobResultArgs{EndTimestamp: &validEndTimestamp, MetricPath: metricPath, StartTimestamp: startTimestamp, Job: job, Pipeline: pipeline, ProjectID: projectID, ResultState: resultState, Task: task, Zone: " "}, "empty zone"},
		{JobResultArgs{EndTimestamp: &validEndTimestamp, MetricPath: metricPath, StartTimestamp: startTimestamp, Job: job, Pipeline: pipeline, ProjectID: projectID, ResultState: resultState, Task: " ", Zone: zone}, "empty task"},
		{JobResultArgs{EndTimestamp: &validEndTimestamp, MetricPath: metricPath, StartTimestamp: startTimestamp, Job: job, Pipeline: pipeline, ProjectID: projectID, ResultState: "other", Task: task, Zone: zone}, "invalid state"},
		{JobResultArgs{EndTimestamp: &validEndTimestamp, MetricPath: metricPath, StartTimestamp: startTimestamp, Job: job, Pipeline: pipeline, ProjectID: " ", ResultState: resultState, Task: task, Zone: zone}, "empty project"},
		{JobResultArgs{EndTimestamp: &validEndTimestamp, MetricPath: metricPath, StartTimestamp: startTimestamp, Job: job, Pipeline: " ", ProjectID: projectID, ResultState: resultState, Task: task, Zone: zone}, "empty pipeline"},
		{JobResultArgs{EndTimestamp: &validEndTimestamp, MetricPath: metricPath, StartTimestamp: startTimestamp, Job: " ", Pipeline: pipeline, ProjectID: projectID, ResultState: resultState, Task: task, Zone: zone}, "empty job"},
		{JobResultArgs{EndTimestamp: &validEndTimestamp, MetricPath: metricPath, StartTimestamp: validEndTimestamp + 1000, Job: job, Pipeline: pipeline, ProjectID: projectID, ResultState: resultState, Task: task, Zone: zone}, "end-timestamp cannot occur before start-timestamp"},
	}
	for _, test := range tests {
		if _, err := BuildJobResultRequest(test.args); err == nil || !strings.Contains(err.Error(), test.message) {
			t.Errorf("BuildJobResultRequest(%+v) = (_, %v); want error to contain %s", test.args, err, test.message)
		}
	}
}

func assertEqual(t *testing.T, x interface{}, y interface{}) {
	if x == y {
		return
	}

	t.Errorf("Received %v (type %v), expected %v (type %v)", x, reflect.TypeOf(x), y, reflect.TypeOf(y))
}
