package config

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestNewSafeConfig(t *testing.T) {
	t.Run("should load configuration", func(t *testing.T) {
		sc := NewSafeConfig(prometheus.NewRegistry())
		err := sc.ReloadConfig("./testdata/config-valid.yaml", slog.Default())

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
		err := sc.ReloadConfig("./testdata/config-invalid.yaml", slog.Default())

		require.Error(t, err)
	})
}

func TestNewSafeConfigFromUrl(t *testing.T) {
	t.Run("should load configuration", func(t *testing.T) {
		configServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `scripts:
  - name: output
    command:
      - ./prober/scripts/output.sh
`)
		}))
		defer configServer.Close()

		sc := NewSafeConfig(prometheus.NewRegistry())
		err := sc.ReloadConfig(configServer.URL, slog.Default())

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

	t.Run("should return error for invalid status code", func(t *testing.T) {
		configServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Not Found", http.StatusNotFound)
		}))
		defer configServer.Close()

		sc := NewSafeConfig(prometheus.NewRegistry())
		err := sc.ReloadConfig(configServer.URL, slog.Default())

		require.Error(t, err)
	})

	t.Run("should return error for invalid configuration", func(t *testing.T) {
		configServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `scripts:
  - name: output
    command: ./prober/scripts/output.sh
`)
		}))
		defer configServer.Close()

		sc := NewSafeConfig(prometheus.NewRegistry())
		err := sc.ReloadConfig(configServer.URL, slog.Default())

		require.Error(t, err)
	})
}
