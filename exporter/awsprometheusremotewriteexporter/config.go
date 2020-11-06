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

package awsprometheusremotewriteexporter

import (
	prw "go.opentelemetry.io/collector/exporter/prometheusremotewriteexporter"
)

// Config defines configuration for Remote Write exporter.
type Config struct {
	// import configuration from Prometheus Remote Write Exporter
	prw.Config `mapstructure:",squash"`

	// AWS Sig V4 configuration options
	AuthSettings AuthSettings `mapstructure:"aws_auth"`
}

// AuthSettings defines AWS authentication configurations for SigningRoundTripper
type AuthSettings struct {
	// region string for AWS Sig V4
	Region string `mapstructure:"region"`
	// service string for AWS Sig V4
	Service string `mapstructure:"service"`
}
