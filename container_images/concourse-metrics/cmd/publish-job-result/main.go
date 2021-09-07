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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	"github.com/GoogleCloudPlatform/guest-test-infra/container_images/concourse-metrics/pkg/requests"
)

var (
	endTimestamp   = flag.Int64("end-timestamp", time.Now().UnixNano()/1000000, "End timestamp of the job run. Defaults to now.")
	job            = flag.String("job", "", "Concourse job name.")
	metricPath     = flag.String("metric-path", "", "Path of the custom metric name to use (custom.googleapis.com/[metric-path]).")
	projectID      = flag.String("project-id", "", "GCP project Id.")
	pipeline       = flag.String("pipeline", "", "Concourse pipeline name.")
	task           = flag.String("task", "", "Concourse task name publishing this metric.")
	resultState    = flag.String("result-state", "", "Concourse job result state ('success' or 'failure')")
	startTimestamp = flag.Int64("start-timestamp", -1, "Start timestamp of the job run.")
	zone           = flag.String("zone", "", "GCP zone.")
)

func main() {
	ctx := context.Background()

	flag.Parse()

	c, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		fmt.Printf("Error creating a new Cloud Monitoring metric client: %+v.\n", err)
		os.Exit(1)
	}
	defer c.Close()

	req, err := requests.BuildJobResultRequest(requests.JobResultArgs{
		EndTimestamp:   endTimestamp,
		Job:            *job,
		MetricPath:     *metricPath,
		Pipeline:       *pipeline,
		ProjectID:      *projectID,
		ResultState:    *resultState,
		StartTimestamp: *startTimestamp,
		Task:           *task,
		Zone:           *zone,
	})
	if err != nil {
		fmt.Printf("Error creating request: %+v.\n", err)
		os.Exit(1)
	}

	err = c.CreateTimeSeries(ctx, req)
	if err != nil {
		fmt.Printf("Failed to write time series data: %+v.\n", err)
		os.Exit(1)
	}
}
