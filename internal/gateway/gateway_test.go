package gateway

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.admiral.io/admiral/internal/config"
)

func TestComputeMaximumTimeout(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Timeouts
		want time.Duration
	}{
		{name: "nil config", cfg: nil, want: 15 * time.Second},
		{name: "default only", cfg: &config.Timeouts{Default: 10 * time.Second}, want: 10 * time.Second},
		{name: "override larger", cfg: &config.Timeouts{
			Default:   10 * time.Second,
			Overrides: []config.TimeoutsEntry{{Timeout: 30 * time.Second}},
		}, want: 30 * time.Second},
		{name: "override smaller", cfg: &config.Timeouts{
			Default:   10 * time.Second,
			Overrides: []config.TimeoutsEntry{{Timeout: 5 * time.Second}},
		}, want: 10 * time.Second},
		{name: "zero override means unlimited", cfg: &config.Timeouts{
			Default:   10 * time.Second,
			Overrides: []config.TimeoutsEntry{{Timeout: 0}},
		}, want: 0},
		{name: "zero default means unlimited", cfg: &config.Timeouts{
			Default: 0,
		}, want: 0},
		{name: "multiple overrides picks largest", cfg: &config.Timeouts{
			Default: 10 * time.Second,
			Overrides: []config.TimeoutsEntry{
				{Timeout: 5 * time.Second},
				{Timeout: 60 * time.Second},
				{Timeout: 30 * time.Second},
			},
		}, want: 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, computeMaximumTimeout(tt.cfg))
		})
	}
}
