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
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/prometheus/prometheus/pkg/value"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

var staleNaNsPage1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 19

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 100
http_requests_total{method="post",code="400"} 5

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 1000
http_request_duration_seconds_bucket{le="0.5"} 1500
http_request_duration_seconds_bucket{le="1"} 2000
http_request_duration_seconds_bucket{le="+Inf"} 2500
http_request_duration_seconds_sum 5000
http_request_duration_seconds_count 2500

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1
rpc_duration_seconds{quantile="0.9"} 5
rpc_duration_seconds{quantile="0.99"} 8
rpc_duration_seconds_sum 5000
rpc_duration_seconds_count 1000
`

var (
	totalScrapes = 10
)

// TestStaleNaNs validates that staleness marker gets generated when the timeseries is no longer present
func TestStaleNaNs(t *testing.T) {
	mockResponses := make([]mockPrometheusResponse, 0)
	for i := 0; i < totalScrapes; i++ {
		if i%2 == 0 {
			mockResponses = append(mockResponses, mockPrometheusResponse{
				code: 200,
				data: staleNaNsPage1,
			})
		} else {
			mockResponses = append(mockResponses, mockPrometheusResponse{
				code: 500,
				data: "",
			})
		}
	}
	targets := []*testData{
		{
			name:         "target1",
			pages:        mockResponses,
			validateFunc: verifyStaleNaNs,
		},
	}
	testComponent(t, targets, nil, false, "", true)
}

func verifyStaleNaNs(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyTotalNumScrapeResults(t, td, resourceMetrics)
	metrics1 := resourceMetrics[0].InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	for i := 0; i < totalScrapes; i++ {
		if i%2 == 0 {
			verifySuccessfulScrape(t, td, resourceMetrics[i], &ts1, i+1)
		} else {
			verifyFailedScrape(t, td, resourceMetrics[i], &ts1, i+1)
		}
	}
}

func verifySuccessfulScrape(t *testing.T, td *testData, resourceMetric *pdata.ResourceMetrics, startTimestamp *pdata.Timestamp, iteration int) {
	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(resourceMetric))
	wantAttributes := td.attributes // should want attribute be part of complete target or each scrape?
	metrics1 := resourceMetric.InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	e1 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(*startTimestamp),
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(*startTimestamp),
						compareTimestamp(ts1),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pdata.MetricDataTypeHistogram),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp are incorrect
						// for Summary and Histogram metrics after a failed scrape
						//compareHistogramStartTimestamp(*startTimestamp),
						compareHistogramTimestamp(ts1),
						compareHistogram(2500, 5000, []uint64{1000, 500, 500, 500}),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pdata.MetricDataTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp are incorrect
						// for Summary and Histogram metrics after a failed scrape
						//compareSummaryStartTimestamp(*startTimestamp),
						compareSummaryTimestamp(ts1),
						compareSummary(1000, 5000, [][]float64{{0.01, 1}, {0.9, 5}, {0.99, 8}}),
					},
				},
			}),
	}
	doCompare(t, fmt.Sprintf("validScrape-scrape-%d", iteration), wantAttributes, resourceMetric, e1)
}

func verifyFailedScrape(t *testing.T, td *testData, resourceMetric *pdata.ResourceMetrics, startTimestamp *pdata.Timestamp, iteration int) {
	// m1 has 4 metrics + 5 internal scraper metrics
	// TODO: Prometheus Receiver Issue- incorrect number of metrics in failed scrape
	//assert.Equal(t, 9, metricsCount(resourceMetric))
	wantAttributes := td.attributes
	allMetrics := getMetrics(resourceMetric)
	assertUp(t, 0, allMetrics)

	// TODO: Remove this skip once OTLP format is directly used in Prometheus Receiver Metric Builder.
	if true {
		t.Log(`Skipping the datapoint flag check for staleness markers, as the current receiver doesnt yet set the flag true for staleNaNs`)
		return
	}
	metrics1 := resourceMetric.InstrumentationLibraryMetrics().At(0).Metrics()

	// for failed scrapes, the order of metrics is random,
	// and it is not necessary to always have Gauge as first metric
	ts1, err := getPointTimeStamp(metrics1)
	require.Nilf(t, err, "Failed to get point timestamp: %v", err)
	e1 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						assertNumberPointFlagNoRecordedValue(),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(*startTimestamp),
						compareTimestamp(ts1),
						assertNumberPointFlagNoRecordedValue(),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(*startTimestamp),
						compareTimestamp(ts1),
						assertNumberPointFlagNoRecordedValue(),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pdata.MetricDataTypeHistogram),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(*startTimestamp),
						compareHistogramTimestamp(ts1),
						assertHistogramPointFlagNoRecordedValue(),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pdata.MetricDataTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(*startTimestamp),
						compareSummaryTimestamp(ts1),
						assertSummaryPointFlagNoRecordedValue(),
					},
				},
			}),
	}
	doCompare(t, fmt.Sprintf("failedScrape-scrape-%d", iteration), wantAttributes, resourceMetric, e1)
}

// getPointTimeStamp gets point timestamp from the first metric in the scrape
func getPointTimeStamp(metrics1 pdata.MetricSlice) (pdata.Timestamp, error) {
	switch metrics1.At(0).DataType() {
	case pdata.MetricDataTypeGauge:
		return metrics1.At(0).Gauge().DataPoints().At(0).Timestamp(), nil
	case pdata.MetricDataTypeSum:
		return metrics1.At(0).Sum().DataPoints().At(0).Timestamp(), nil
	case pdata.MetricDataTypeHistogram:
		return metrics1.At(0).Histogram().DataPoints().At(0).Timestamp(), nil
	case pdata.MetricDataTypeSummary:
		return metrics1.At(0).Summary().DataPoints().At(0).Timestamp(), nil
	}
	return 0, errors.New("unknown data type")
}

// Prometheus gauge metric can be set to NaN, a use case could be when value 0 is not representable
// Prometheus summary metric quantiles can have NaN after getting expired
var normalNaNsPage1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads NaN

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} NaN
rpc_duration_seconds{quantile="0.9"} NaN
rpc_duration_seconds{quantile="0.99"} NaN
rpc_duration_seconds_sum 5000
rpc_duration_seconds_count 1000
`

// TestNormalNaNs validates the output of receiver when testdata contains NaN values
func TestNormalNaNs(t *testing.T) {
	// 1. setup input data
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: normalNaNsPage1},
			},
			validateFunc: verifyNormalNaNs,
		},
	}
	testComponent(t, targets, nil, false, "", false)
}

func verifyNormalNaNs(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyValidNumScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	e1 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						assertNormalNan(),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pdata.MetricDataTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts1),
						compareSummary(1000, 5000, [][]float64{{0.01, math.Float64frombits(value.NormalNaN)},
							{0.9, math.Float64frombits(value.NormalNaN)}, {0.99, math.Float64frombits(value.NormalNaN)}}),
					},
				},
			}),
	}
	doCompare(t, "scrape-NormalNaN-1", wantAttributes, m1, e1)
}

var infPage1 = `
# HELP go_threads_max Number of OS threads created
# TYPE go_threads_max gauge
go_threads_max{method="post",code="200"} +Inf

# HELP go_threads_min Number of OS threads created
# TYPE go_threads_min gauge
go_threads_min{method="post",code="400"} -Inf
`

func TestInfValues(t *testing.T) {
	// 1. setup input data
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: infPage1},
			},
			validateFunc: verifyInfValues,
		},
	}
	testComponent(t, targets, nil, false, "", false)
}

func verifyInfValues(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyValidNumScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	e1 := []testExpectation{
		assertMetricPresent("go_threads_max",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
						compareDoubleValue(math.Inf(1)),
					},
				},
			}),
		assertMetricPresent("go_threads_min",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
						compareDoubleValue(math.Inf(-1)),
					},
				},
			}),
	}
	doCompare(t, "scrape-InfValues-1", wantAttributes, m1, e1)
}
