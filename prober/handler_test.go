package prober

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ricoberger/script_exporter/config"

	"github.com/stretchr/testify/require"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

func TestHandler(t *testing.T) {
	t.Run("should return metrics", func(t *testing.T) {
		var c = config.Config{
			Scripts: []config.Script{{
				Name:    "test",
				Command: []string{"sleep"},
				Args:    []string{"1"},
			}},
		}

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/probe?script=test", nil)
		w := httptest.NewRecorder()

		Handler(w, req, &c, logger, false, 0.5, false)

		res := w.Result()
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Contains(t, string(data), `script_success{script="test"} 1`)
		require.Contains(t, string(data), `script_duration_seconds{script="test"}`)
		require.Contains(t, string(data), `script_exit_code{script="test"} 0`)
		require.Contains(t, string(data), `script_cached{script="test"} 0`)
	})

	t.Run("should return error if script is not found", func(t *testing.T) {
		var c = config.Config{}

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/probe?script=test", nil)
		w := httptest.NewRecorder()

		Handler(w, req, &c, logger, false, 0.5, false)

		res := w.Result()
		defer res.Body.Close()

		require.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("should use max timeout", func(t *testing.T) {
		var c = config.Config{
			Scripts: []config.Script{{
				Name:    "test",
				Command: []string{"sleep"},
				Args:    []string{"5"},
				Timeout: config.Timeout{
					MaxTimeout: 1,
					Enforced:   true,
				},
			}},
		}

		startTime := time.Now()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/probe?script=test", nil)
		w := httptest.NewRecorder()

		Handler(w, req, &c, logger, false, 0.5, false)

		res := w.Result()
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Contains(t, string(data), `script_success{script="test"} 0`)
		require.Contains(t, string(data), `script_duration_seconds{script="test"}`)
		require.Contains(t, string(data), `script_exit_code{script="test"} -1`)
		require.Contains(t, string(data), `script_cached{script="test"} 0`)
		require.Less(t, time.Since(startTime).Seconds(), float64(2))
	})

	t.Run("should not enforce max timeout when wait delay is not set", func(t *testing.T) {
		var c = config.Config{
			Scripts: []config.Script{{
				Name:    "test",
				Command: []string{"./scripts/sleep.sh"},
				Args:    []string{"5"},
				Timeout: config.Timeout{
					MaxTimeout: 1,
					Enforced:   true,
				},
			}},
		}

		startTime := time.Now()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/probe?script=test", nil)
		w := httptest.NewRecorder()

		Handler(w, req, &c, logger, false, 0.5, false)

		res := w.Result()
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Contains(t, string(data), `script_success{script="test"} 0`)
		require.Contains(t, string(data), `script_duration_seconds{script="test"}`)
		require.Contains(t, string(data), `script_exit_code{script="test"} -1`)
		require.Contains(t, string(data), `script_cached{script="test"} 0`)
		require.Greater(t, time.Since(startTime).Seconds(), float64(2))
	})

	t.Run("should enforce max timeout when wait delay is set", func(t *testing.T) {
		var c = config.Config{
			Scripts: []config.Script{{
				Name:    "test",
				Command: []string{"./scripts/sleep.sh"},
				Args:    []string{"5"},
				Timeout: config.Timeout{
					MaxTimeout: 1,
					Enforced:   true,
					WaitDelay:  0.01,
				},
			}},
		}

		startTime := time.Now()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/probe?script=test", nil)
		w := httptest.NewRecorder()

		Handler(w, req, &c, logger, false, 0.5, false)

		res := w.Result()
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Contains(t, string(data), `script_success{script="test"} 0`)
		require.Contains(t, string(data), `script_duration_seconds{script="test"}`)
		require.Contains(t, string(data), `script_exit_code{script="test"} -1`)
		require.Contains(t, string(data), `script_cached{script="test"} 0`)
		require.Less(t, time.Since(startTime).Seconds(), float64(2))
	})

	t.Run("should use parameters as arguments and set environemnt variables", func(t *testing.T) {
		var c = config.Config{
			Scripts: []config.Script{{
				Name:    "test",
				Command: []string{"sleep"},
				Env:     map[string]string{"HELOL": "WORLD"},
			}},
		}

		startTime := time.Now()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/probe?script=test&params=seconds&seconds=5", nil)
		w := httptest.NewRecorder()

		Handler(w, req, &c, logger, false, 0.5, false)

		res := w.Result()
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Contains(t, string(data), `script_success{script="test"} 1`)
		require.Contains(t, string(data), `script_duration_seconds{script="test"}`)
		require.Contains(t, string(data), `script_exit_code{script="test"} 0`)
		require.Contains(t, string(data), `script_cached{script="test"} 0`)
		require.Greater(t, time.Since(startTime).Seconds(), float64(5))
	})

	t.Run("should cache result", func(t *testing.T) {
		cacheDuration := float64(10)

		var c = config.Config{
			Scripts: []config.Script{{
				Name:    "test",
				Command: []string{"sleep"},
				Args:    []string{"5"},
				Cache: config.Cache{
					Duration: &cacheDuration,
				},
			}},
		}

		// Uncached
		startTime1 := time.Now()
		req1, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/probe?script=test", nil)
		w1 := httptest.NewRecorder()

		Handler(w1, req1, &c, logger, false, 0.5, false)

		res1 := w1.Result()
		defer res1.Body.Close()

		require.Equal(t, http.StatusOK, res1.StatusCode)
		require.Greater(t, time.Since(startTime1).Seconds(), float64(5))

		// Cached
		startTime2 := time.Now()
		req2, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/probe?script=test", nil)
		w2 := httptest.NewRecorder()

		Handler(w2, req2, &c, logger, false, 0.5, false)

		res2 := w2.Result()
		defer res2.Body.Close()

		require.Equal(t, http.StatusOK, res2.StatusCode)
		require.Less(t, time.Since(startTime2).Seconds(), float64(2))
	})
}
