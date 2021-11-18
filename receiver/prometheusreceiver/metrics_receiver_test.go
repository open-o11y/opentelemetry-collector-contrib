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
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Test data and validation functions for all four core metrics for Prometheus Receiver.
// Make sure every page has a gauge, we are relying on it to figure out the start time if needed

// target1 has one gauge, two counts of a same family, one histogram and one summary. We are expecting the both
// successful scrapes will produce all metrics using the first scrape's timestamp as start time.

// With the 2nd page, we are simulating a reset (values smaller than previous), start_timestamp should be from
// this run for the 2nd scrape, despite a failed scrape between 1st and 2nd page.

var target1Page1 = `
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

var target1Page2 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 18

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 99
http_requests_total{method="post",code="400"} 3

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 900
http_request_duration_seconds_bucket{le="0.5"} 1400
http_request_duration_seconds_bucket{le="1"} 1900
http_request_duration_seconds_bucket{le="+Inf"} 2400
http_request_duration_seconds_sum 4950
http_request_duration_seconds_count 2400

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1
rpc_duration_seconds{quantile="0.9"} 6
rpc_duration_seconds{quantile="0.99"} 8
rpc_duration_seconds_sum 4980
rpc_duration_seconds_count 900
`

func verifyTarget1(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyNumScrapeResults(t, td, resourceMetrics)
	require.Greater(t, len(resourceMetrics), 0, "At least one resource metric should be present")
	m1 := resourceMetrics[0]

	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m1))

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
						compareDoubleValue(19),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
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
						compareHistogramStartTimestamp(ts1),
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
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts1),
						compareSummary(1000, 5000, [][]float64{{0.01, 1}, {0.9, 5}, {0.99, 8}}),
					},
				},
			}),
	}
	doCompare(t, "scrape1", wantAttributes, m1, e1)

	m2 := resourceMetrics[1]
	// m2 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m2))

	metricsScrape2 := m2.InstrumentationLibraryMetrics().At(0).Metrics()
	ts2 := metricsScrape2.At(0).Gauge().DataPoints().At(0).Timestamp()
	e2 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts2),
						compareDoubleValue(18),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp should reset after a failed scrape, if the scrape prior to the failed scrape had higher value
						//compareStartTimestamp(ts2),
						compareTimestamp(ts2),
						compareDoubleValue(99),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp should reset after a failed scrape, if the scrape prior to the failed scrape had higher value
						//compareStartTimestamp(ts2),
						compareTimestamp(ts2),
						compareDoubleValue(3),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pdata.MetricDataTypeHistogram),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp are incorrect for Histogram metrics after a failed scrape
						//compareHistogramStartTimestamp(ts2),
						compareHistogramTimestamp(ts2),
						compareHistogram(2400, 4950, []uint64{900, 500, 500, 500}),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pdata.MetricDataTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp are incorrect for Summary metrics after a failed scrape
						//compareSummaryStartTimestamp(ts2),
						compareSummaryTimestamp(ts2),
						compareSummary(900, 4980, [][]float64{{0.01, 1}, {0.9, 6}, {0.99, 8}}),
					},
				},
			}),
	}
	doCompare(t, "scrape2", wantAttributes, m2, e2)
}

// target2 is going to have 5 pages, and there's a newly added item on the 2nd page.
// with the 4th page, we are simulating a reset (values smaller than previous), start times should be from
// this run for the 4th and 5th scrapes.

var target2Page1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 18

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 10
http_requests_total{method="post",code="400"} 50
`

var target2Page2 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 50
http_requests_total{method="post",code="400"} 60
http_requests_total{method="post",code="500"} 3
`

var target2Page3 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 50
http_requests_total{method="post",code="400"} 60
http_requests_total{method="post",code="500"} 5
`

var target2Page4 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 49
http_requests_total{method="post",code="400"} 59
http_requests_total{method="post",code="500"} 3
`

var target2Page5 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 50
http_requests_total{method="post",code="400"} 59
http_requests_total{method="post",code="500"} 5
`

func verifyTarget2(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyNumScrapeResults(t, td, resourceMetrics)
	require.Greater(t, len(resourceMetrics), 0, "At least one resource metric should be present")
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
						compareDoubleValue(18),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts1),
						compareDoubleValue(10),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts1),
						compareDoubleValue(50),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			}),
	}
	doCompare(t, "scrape1", wantAttributes, m1, e1)

	m2 := resourceMetrics[1]
	// m2 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m2))

	metricsScrape2 := m2.InstrumentationLibraryMetrics().At(0).Metrics()
	ts2 := metricsScrape2.At(0).Gauge().DataPoints().At(0).Timestamp()
	e2 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts2),
						compareDoubleValue(16),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts2),
						compareDoubleValue(50),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts2),
						compareDoubleValue(60),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts2),
						compareTimestamp(ts2),
						compareDoubleValue(3),
						compareAttributes(map[string]string{"method": "post", "code": "500"}),
					},
				},
			}),
	}
	doCompare(t, "scrape2", wantAttributes, m2, e2)

	m3 := resourceMetrics[2]
	// m3 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m3))

	metricsScrape3 := m3.InstrumentationLibraryMetrics().At(0).Metrics()
	ts3 := metricsScrape3.At(0).Gauge().DataPoints().At(0).Timestamp()
	e3 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts3),
						compareDoubleValue(16),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts3),
						compareDoubleValue(50),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts3),
						compareDoubleValue(60),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts2),
						compareTimestamp(ts3),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "500"}),
					},
				},
			}),
	}
	doCompare(t, "scrape3", wantAttributes, m3, e3)

	m4 := resourceMetrics[3]
	// m4 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m4))

	metricsScrape4 := m4.InstrumentationLibraryMetrics().At(0).Metrics()
	ts4 := metricsScrape4.At(0).Gauge().DataPoints().At(0).Timestamp()
	e4 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts4),
						compareDoubleValue(16),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts4),
						compareTimestamp(ts4),
						compareDoubleValue(49),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts4),
						compareTimestamp(ts4),
						compareDoubleValue(59),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts4),
						compareTimestamp(ts4),
						compareDoubleValue(3),
						compareAttributes(map[string]string{"method": "post", "code": "500"}),
					},
				},
			}),
	}
	doCompare(t, "scrape4", wantAttributes, m4, e4)

	m5 := resourceMetrics[4]
	// m5 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m5))

	metricsScrape5 := m5.InstrumentationLibraryMetrics().At(0).Metrics()
	ts5 := metricsScrape5.At(0).Gauge().DataPoints().At(0).Timestamp()
	e5 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts5),
						compareDoubleValue(16),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts4),
						compareTimestamp(ts5),
						compareDoubleValue(50),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts4),
						compareTimestamp(ts5),
						compareDoubleValue(59),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts4),
						compareTimestamp(ts5),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "500"}),
					},
				},
			}),
	}
	doCompare(t, "scrape5", wantAttributes, m5, e5)
}

// target3 for complicated data types, including summaries and histograms. one of the summary and histogram have only
// sum/count, for the summary it's valid, however the histogram one is not, but it shall not cause the scrape to fail

var target3Page1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 18

# A histogram, which has a pretty complex representation in the text format:
# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.2"} 10000
http_request_duration_seconds_bucket{le="0.5"} 11000
http_request_duration_seconds_bucket{le="1"} 12001
http_request_duration_seconds_bucket{le="+Inf"} 13003
http_request_duration_seconds_sum 50000
http_request_duration_seconds_count 13003

# A corrupted histogram with only sum and count
# HELP corrupted_hist A corrupted_hist.
# TYPE corrupted_hist histogram
corrupted_hist_sum 100
corrupted_hist_count 10

# Finally a summary, which has a complex representation, too:
# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{foo="bar" quantile="0.01"} 31
rpc_duration_seconds{foo="bar" quantile="0.05"} 35
rpc_duration_seconds{foo="bar" quantile="0.5"} 47
rpc_duration_seconds{foo="bar" quantile="0.9"} 70
rpc_duration_seconds{foo="bar" quantile="0.99"} 76
rpc_duration_seconds_sum{foo="bar"} 8000
rpc_duration_seconds_count{foo="bar"} 900
rpc_duration_seconds_sum{foo="no_quantile"} 100
rpc_duration_seconds_count{foo="no_quantile"} 50
`

var target3Page2 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# A histogram, which has a pretty complex representation in the text format:
# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.2"} 11000
http_request_duration_seconds_bucket{le="0.5"} 12000
http_request_duration_seconds_bucket{le="1"} 13001
http_request_duration_seconds_bucket{le="+Inf"} 14003
http_request_duration_seconds_sum 50100
http_request_duration_seconds_count 14003

# A corrupted histogram with only sum and count	
# HELP corrupted_hist A corrupted_hist.
# TYPE corrupted_hist histogram
corrupted_hist_sum 101
corrupted_hist_count 15

# Finally a summary, which has a complex representation, too:
# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{foo="bar" quantile="0.01"} 32
rpc_duration_seconds{foo="bar" quantile="0.05"} 35
rpc_duration_seconds{foo="bar" quantile="0.5"} 47
rpc_duration_seconds{foo="bar" quantile="0.9"} 70
rpc_duration_seconds{foo="bar" quantile="0.99"} 77
rpc_duration_seconds_sum{foo="bar"} 8100
rpc_duration_seconds_count{foo="bar"} 950
rpc_duration_seconds_sum{foo="no_quantile"} 101
rpc_duration_seconds_count{foo="no_quantile"} 55
`

// target3Page3 has higher value than previous scrapes.
// So, even after seeing a failed scrape, start_timestamp should not be reset for target3page3
// Start_timestamp should be of the initial scrape for target3page3
var target3Page3 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 19

# A histogram, which has a pretty complex representation in the text format:
# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.2"} 11000
http_request_duration_seconds_bucket{le="0.5"} 13000
http_request_duration_seconds_bucket{le="1"} 14001
http_request_duration_seconds_bucket{le="+Inf"} 14003
http_request_duration_seconds_sum 50500
http_request_duration_seconds_count 14005

# A corrupted histogram with only sum and count	
# HELP corrupted_hist A corrupted_hist.
# TYPE corrupted_hist histogram
corrupted_hist_sum 110
corrupted_hist_count 19

# Finally a summary, which has a complex representation, too:
# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{foo="bar" quantile="0.01"} 36
rpc_duration_seconds{foo="bar" quantile="0.05"} 35
rpc_duration_seconds{foo="bar" quantile="0.5"} 52
rpc_duration_seconds{foo="bar" quantile="0.9"} 70
rpc_duration_seconds{foo="bar" quantile="0.99"} 77
rpc_duration_seconds_sum{foo="bar"} 8900
rpc_duration_seconds_count{foo="bar"} 990
rpc_duration_seconds_sum{foo="no_quantile"} 109
rpc_duration_seconds_count{foo="no_quantile"} 59
`

func verifyTarget3(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyNumScrapeResults(t, td, resourceMetrics)
	require.Equal(t, len(resourceMetrics), 3, "Three resource metrics should be present")
	wantAttributes := td.attributes

	m1 := resourceMetrics[0]
	// m1 has 3 metrics + 5 internal scraper metrics
	assert.Equal(t, 8, metricsCount(m1))
	metrics1 := m1.InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	e1 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(18),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pdata.MetricDataTypeHistogram),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(ts1),
						compareHistogramTimestamp(ts1),
						compareHistogram(13003, 50000, []uint64{10000, 1000, 1001, 1002}),
					},
				},
			}),
		assertMetricAbsent("corrupted_hist"),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pdata.MetricDataTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts1),
						compareSummaryAttributes(map[string]string{"foo": "bar"}),
						compareSummary(900, 8000, [][]float64{{0.01, 31}, {0.05, 35}, {0.5, 47}, {0.9, 70}, {0.99, 76}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts1),
						compareSummaryAttributes(map[string]string{"foo": "no_quantile"}),
						compareSummary(50, 100, [][]float64{}),
					},
				},
			}),
	}
	doCompare(t, "scrape1", wantAttributes, m1, e1)

	m2 := resourceMetrics[1]
	// m2 has 3 metrics + 5 internal scraper metrics
	assert.Equal(t, 8, metricsCount(m2))
	metricsScrape2 := m2.InstrumentationLibraryMetrics().At(0).Metrics()
	ts2 := metricsScrape2.At(0).Gauge().DataPoints().At(0).Timestamp()
	e2 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts2),
						compareDoubleValue(16),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pdata.MetricDataTypeHistogram),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(ts1),
						compareHistogramTimestamp(ts2),
						compareHistogram(14003, 50100, []uint64{11000, 1000, 1001, 1002}),
					},
				},
			}),
		assertMetricAbsent("corrupted_hist"),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pdata.MetricDataTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts2),
						compareSummaryAttributes(map[string]string{"foo": "bar"}),
						compareSummary(950, 8100, [][]float64{{0.01, 32}, {0.05, 35}, {0.5, 47}, {0.9, 70}, {0.99, 77}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts2),
						compareSummaryAttributes(map[string]string{"foo": "no_quantile"}),
						compareSummary(55, 101, [][]float64{}),
					},
				},
			}),
	}
	doCompare(t, "scrape2", wantAttributes, m2, e2)

	m3 := resourceMetrics[2]
	// m2 has 3 metrics + 5 internal scraper metrics
	assert.Equal(t, 8, metricsCount(m3))
	metricsScrape3 := m3.InstrumentationLibraryMetrics().At(0).Metrics()
	ts3 := metricsScrape3.At(0).Gauge().DataPoints().At(0).Timestamp()
	e3 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts3),
						compareDoubleValue(19),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pdata.MetricDataTypeHistogram),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp are incorrect for Histogram metrics after a failed scrape
						// Page 3 start_timestamp should not get reset only because of a prior failed scrape
						//compareHistogramStartTimestamp(ts1),
						compareHistogramTimestamp(ts3),
						compareHistogram(14005, 50500, []uint64{11000, 2000, 1001, 2}),
					},
				},
			}),
		assertMetricAbsent("corrupted_hist"),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pdata.MetricDataTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp are incorrect for Summary metrics after a failed scrape
						// Page 3 start_timestamp should not get reset only because of a prior failed scrape
						//compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts3),
						compareSummaryAttributes(map[string]string{"foo": "bar"}),
						compareSummary(990, 8900, [][]float64{{0.01, 36}, {0.05, 35}, {0.5, 52}, {0.9, 70}, {0.99, 77}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp are incorrect for Summary metrics after a failed scrape
						// Page 3 start_timestamp should not get reset only because of a prior failed scrape
						//compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts3),
						compareSummaryAttributes(map[string]string{"foo": "no_quantile"}),
						compareSummary(59, 109, [][]float64{}),
					},
				},
			}),
	}
	doCompare(t, "scrape3", wantAttributes, m3, e3)
}

// TestCoreMetricsEndToEnd end to end test executor
func TestCoreMetricsEndToEnd(t *testing.T) {
	// 1. setup input data
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: target1Page1},
				{code: 500, data: ""},
				{code: 200, data: target1Page2},
			},
			validateFunc: verifyTarget1,
		},
		{
			name: "target2",
			pages: []mockPrometheusResponse{
				{code: 200, data: target2Page1},
				{code: 200, data: target2Page2},
				{code: 500, data: ""},
				{code: 200, data: target2Page3},
				{code: 200, data: target2Page4},
				{code: 500, data: ""},
				{code: 200, data: target2Page5},
			},
			validateFunc: verifyTarget2,
		},
		{
			name: "target3",
			pages: []mockPrometheusResponse{
				{code: 200, data: target3Page1},
				{code: 200, data: target3Page2},
				{code: 500, data: ""},
				{code: 200, data: target3Page3},
			},
			validateFunc: verifyTarget3,
		},
	}
	testComponent(t, targets, nil, false, "")
}

var startTimeMetricPage = `
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
# HELP process_start_time_seconds Start time of the process since unix epoch in seconds.
# TYPE process_start_time_seconds gauge
process_start_time_seconds 400.8
`

var startTimeMetricPageStartTimestamp = &timestamppb.Timestamp{Seconds: 400, Nanos: 800000000}

// 6 metrics + 5 internal metrics
const numStartTimeMetricPageTimeseries = 11

func verifyStartTimeMetricPage(t *testing.T, td *testData, result []*pdata.ResourceMetrics) {
	verifyNumScrapeResults(t, td, result)
	numTimeseries := 0
	for _, rm := range result {
		metrics := getMetrics(rm)
		for i := 0; i < len(metrics); i++ {
			timestamp := startTimeMetricPageStartTimestamp
			switch metrics[i].DataType() {
			case pdata.MetricDataTypeGauge:
				timestamp = nil
				for j := 0; j < metrics[i].Gauge().DataPoints().Len(); j++ {
					time := timestamppb.New(metrics[i].Gauge().DataPoints().At(j).StartTimestamp().AsTime())
					assert.Equal(t, timestamp.AsTime(), time.AsTime())
					numTimeseries++
				}

			case pdata.MetricDataTypeSum:
				for j := 0; j < metrics[i].Sum().DataPoints().Len(); j++ {
					assert.Equal(t, timestamp.AsTime(), timestamppb.New(metrics[i].Sum().DataPoints().At(j).StartTimestamp().AsTime()).AsTime())
					numTimeseries++
				}

			case pdata.MetricDataTypeHistogram:
				for j := 0; j < metrics[i].Histogram().DataPoints().Len(); j++ {
					assert.Equal(t, timestamp.AsTime(), timestamppb.New(metrics[i].Histogram().DataPoints().At(j).StartTimestamp().AsTime()).AsTime())
					numTimeseries++
				}

			case pdata.MetricDataTypeSummary:
				for j := 0; j < metrics[i].Summary().DataPoints().Len(); j++ {
					assert.Equal(t, timestamp.AsTime(), timestamppb.New(metrics[i].Summary().DataPoints().At(j).StartTimestamp().AsTime()).AsTime())
					numTimeseries++
				}
			}
		}
		assert.Equal(t, numStartTimeMetricPageTimeseries, numTimeseries)
	}
}

// TestStartTimeMetric validates that timeseries have start time set to 'process_start_time_seconds'
func TestStartTimeMetric(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: startTimeMetricPage},
			},
			validateFunc: verifyStartTimeMetricPage,
		},
	}

	testComponent(t, targets, nil, true, "")
}

var startTimeMetricRegexPage = `
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
# HELP example_process_start_time_seconds Start time of the process since unix epoch in seconds.
# TYPE example_process_start_time_seconds gauge
example_process_start_time_seconds 400.8
`

// TestStartTimeMetricRegex validates that timeseries have start time regex set to 'process_start_time_seconds'
func TestStartTimeMetricRegex(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: startTimeMetricRegexPage},
			},
			validateFunc: verifyStartTimeMetricPage,
		},
		{
			name: "target2",
			pages: []mockPrometheusResponse{
				{code: 200, data: startTimeMetricPage},
			},
			validateFunc: verifyStartTimeMetricPage,
		},
	}

	testComponent(t, targets, nil, true, "^(.+_)*process_start_time_seconds$")
}
