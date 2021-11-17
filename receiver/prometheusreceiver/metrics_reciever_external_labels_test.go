// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusreceiver

import (
	"testing"

	"github.com/prometheus/prometheus/pkg/labels"
	"go.opentelemetry.io/collector/model/pdata"
)

const targetExternalLabels = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 19`

func TestExternalLabels(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetExternalLabels},
			},
			validateFunc: verifyExternalLabels,
		},
	}

	pConfig := &promConfig{
		externalLabels: []labels.Label{
			{
				Name:  "key",
				Value: "value",
			},
		},
	}
	testComponent(t, targets, pConfig, false, "")
}

func verifyExternalLabels(t *testing.T, td *testData, rms []*pdata.ResourceMetrics) {
	verifyNumScrapeResults(t, td, rms)
	if len(rms) < 1 {
		t.Fatal("At least one metric request should be present")
	}
	wantAttributes := td.attributes
	metrics1 := rms[0].InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	doCompare(t, "scrape-externalLabels", wantAttributes, rms[0], []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
						compareAttributes(map[string]string{"key": "value"}),
					},
				},
			}),
	})
}
