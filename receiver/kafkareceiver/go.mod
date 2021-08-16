module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver

go 1.16

require (
	github.com/Shopify/sarama v1.29.1
	github.com/apache/thrift v0.14.2
	github.com/gogo/protobuf v1.3.2
	github.com/jaegertracing/jaeger v1.25.0
	github.com/openzipkin/zipkin-go v0.2.5
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.32.0
	go.opentelemetry.io/collector/model v0.32.0
	go.uber.org/zap v1.19.0
)
