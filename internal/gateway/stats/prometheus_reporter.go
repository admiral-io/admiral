package stats

import (
	tallyprom "github.com/uber-go/tally/v4/prometheus"
)

func NewPrometheusReporter() (tallyprom.Reporter, error) {
	promCfg := tallyprom.Configuration{}
	return promCfg.NewReporter(tallyprom.ConfigurationOptions{})
}
