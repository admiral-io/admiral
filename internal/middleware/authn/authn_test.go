package authn

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBearerToken(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		want    string
		wantErr string
	}{
		{name: "valid", header: "Bearer abc123", want: "abc123"},
		{name: "case insensitive", header: "bearer abc123", want: "abc123"},
		{name: "BEARER uppercase", header: "BEARER abc123", want: "abc123"},
		{name: "missing scheme", header: "abc123", wantErr: "bad token format"},
		{name: "wrong scheme", header: "Basic abc123", wantErr: "bad token format"},
		{name: "empty header", header: "", wantErr: "bad token format"},
		{name: "bearer only", header: "Bearer", wantErr: "bad token format"},
		{name: "extra fields", header: "Bearer abc 123", wantErr: "bad token format"},
		{name: "admiral pat token", header: "Bearer admp_abc123", want: "admp_abc123"},
		{name: "admiral sat token", header: "Bearer adms_abc123", want: "adms_abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBearerToken(tt.header)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
