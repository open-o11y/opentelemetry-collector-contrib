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

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

// metric type is defined as 'untyped' in the first metric
// and, type hint is missing in the 2nd metric
var untypedMetrics = `
# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total untyped
http_requests_total{method="post",code="200"} 100
http_requests_total{method="post",code="400"} 5

# HELP redis_connected_clients Redis connected clients
redis_connected_clients{name="rough-snowflake-web",port="6380"} 10.0
redis_connected_clients{name="rough-snowflake-web",port="6381"} 12.0
`

// TestUntypedMetrics validates the pass through of untyped metrics through metric receiver and the conversion of untyped to gauge double
func TestUntypedMetrics(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: untypedMetrics},
			},
			validateFunc: verifyUntypedMetrics,
		},
	}

	testComponent(t, targets, nil, false, "")
}

func verifyUntypedMetrics(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyNumScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	e1 := []testExpectation{
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			}),
		assertMetricPresent("redis_connected_clients",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(10),
						compareAttributes(map[string]string{"name": "rough-snowflake-web", "port": "6380"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(12),
						compareAttributes(map[string]string{"name": "rough-snowflake-web", "port": "6381"}),
					},
				},
			}),
	}
	doCompare(t, "scrape-untypedMetric-1", wantAttributes, m1, e1)
}
