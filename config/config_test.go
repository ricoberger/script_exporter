package config

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestNewSafeConfig(t *testing.T) {
	t.Run("should load configuration", func(t *testing.T) {
		sc := NewSafeConfig(prometheus.NewRegistry())
		err := sc.ReloadConfig("./testdata/config-valid.yaml", nil)

		require.NoError(t, err)
		require.NotNil(t, sc.C)

		t.Run("should return script", func(t *testing.T) {
			script := sc.C.GetScript("output")
			require.NotNil(t, script)
			require.Equal(t, "output", script.Name)
		})

		t.Run("should return nil if script is not found", func(t *testing.T) {
			script := sc.C.GetScript("invalid")
			require.Nil(t, script)
		})
	})

	t.Run("should return error for invalid configuration", func(t *testing.T) {
		sc := NewSafeConfig(prometheus.NewRegistry())
		err := sc.ReloadConfig("./testdata/config-invalid.yaml", nil)

		require.Error(t, err)
	})
}
