package discovery

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ricoberger/script_exporter/config"

	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	t.Run("should return targets", func(t *testing.T) {
		var c = config.Config{
			Scripts: []config.Script{{
				Name:    "test",
				Command: []string{"test"},
			}},
		}

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:9469/dicovery", nil)
		w := httptest.NewRecorder()

		Handler(w, req, &c, nil, "", "", "", "")

		res := w.Result()
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)

		require.NoError(t, err)
		require.Equal(t, res.StatusCode, http.StatusOK)
		require.Equal(t, "[{\"targets\":[\"localhost:9469\"],\"labels\":{\"__metrics_path__\":\"/probe\",\"__param_params\":\"\",\"__param_script\":\"test\",\"__scheme__\":\"http\"}}]", string(data))
	})

	t.Run("should use discovery configuration", func(t *testing.T) {
		var c = config.Config{
			Scripts: []config.Script{{
				Name:    "test",
				Command: []string{"test"},
				Discovery: config.Discovery{
					Params:         map[string]string{"seconds": "5"},
					ScrapeInterval: "10s",
					ScrapeTimeout:  "5s",
				},
			}},
		}

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:9469/dicovery", nil)
		w := httptest.NewRecorder()

		Handler(w, req, &c, nil, "", "", "", "")

		res := w.Result()
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)

		require.NoError(t, err)
		require.Equal(t, res.StatusCode, http.StatusOK)
		require.Equal(t, "[{\"targets\":[\"localhost:9469\"],\"labels\":{\"__metrics_path__\":\"/probe\",\"__param_params\":\"seconds\",\"__param_script\":\"test\",\"__param_seconds\":\"5\",\"__scheme__\":\"http\",\"__scrape_interval__\":\"10s\",\"__scrape_timeout__\":\"5s\"}}]", string(data))
	})

	t.Run("should use discovery flags", func(t *testing.T) {
		var c = config.Config{
			Scripts: []config.Script{{
				Name:    "test",
				Command: []string{"test"},
			}},
		}

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:9469/dicovery", nil)
		w := httptest.NewRecorder()

		Handler(w, req, &c, nil, "script_exporter", "9999", "https", "/prefix")

		res := w.Result()
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)

		require.NoError(t, err)
		require.Equal(t, res.StatusCode, http.StatusOK)
		require.Equal(t, "[{\"targets\":[\"script_exporter:9999\"],\"labels\":{\"__metrics_path__\":\"/prefix/probe\",\"__param_params\":\"\",\"__param_script\":\"test\",\"__scheme__\":\"https\"}}]", string(data))
	})

	t.Run("should use default port", func(t *testing.T) {
		var c = config.Config{
			Scripts: []config.Script{{
				Name:    "test",
				Command: []string{"test"},
			}},
		}

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://script_exporter/dicovery", nil)
		w := httptest.NewRecorder()

		Handler(w, req, &c, nil, "", "", "", "")

		res := w.Result()
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)

		require.NoError(t, err)
		require.Equal(t, res.StatusCode, http.StatusOK)
		require.Equal(t, "[{\"targets\":[\"script_exporter:9469\"],\"labels\":{\"__metrics_path__\":\"/probe\",\"__param_params\":\"\",\"__param_script\":\"test\",\"__scheme__\":\"http\"}}]", string(data))
	})
}
