package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestServerSetDefaults(t *testing.T) {
	s := &Server{}
	s.SetDefaults()

	assert.Equal(t, "0.0.0.0", s.Listener.Address)
	assert.Equal(t, 8080, s.Listener.Port)
	require.NotNil(t, s.Logger)
	assert.Equal(t, zap.ErrorLevel, s.Logger.Level)
	require.NotNil(t, s.Stats)
	assert.Equal(t, ReporterTypeNull, s.Stats.ReporterType)
	assert.Equal(t, "admiral", s.Stats.Prefix)
}

func TestServerSetDefaults_PreservesExisting(t *testing.T) {
	s := &Server{
		Listener: Listener{Address: "127.0.0.1", Port: 9090},
	}
	s.SetDefaults()

	assert.Equal(t, "127.0.0.1", s.Listener.Address)
	assert.Equal(t, 9090, s.Listener.Port)
}

func TestServerSetDefaults_NilReceiver(t *testing.T) {
	var s *Server
	s.SetDefaults() // should not panic
}

func TestServerValidate(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var s *Server
		assert.NoError(t, s.Validate())
	})

	t.Run("valid stats", func(t *testing.T) {
		s := &Server{Stats: &Stats{ReporterType: ReporterTypePrometheus}}
		assert.NoError(t, s.Validate())
	})

	t.Run("invalid reporter type", func(t *testing.T) {
		s := &Server{Stats: &Stats{ReporterType: "bogus"}}
		assert.Error(t, s.Validate())
	})
}

func TestReporterTypeValidate(t *testing.T) {
	tests := []struct {
		rt      ReporterType
		wantErr bool
	}{
		{ReporterTypeNull, false},
		{ReporterTypeLog, false},
		{ReporterTypePrometheus, false},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			err := tt.rt.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListenerSetDefaults(t *testing.T) {
	l := &Listener{}
	l.SetDefaults()
	assert.Equal(t, "0.0.0.0", l.Address)
	assert.Equal(t, 8080, l.Port)
}
