package discovery

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/ricoberger/script_exporter/config"
)

type target struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func Handler(w http.ResponseWriter, r *http.Request, c *config.Config, logger *slog.Logger, discoveryHost string, discoveryPort string, discoveryScheme string, routePrefix string) {
	w.Header().Set("Content-Type", "application/json")

	host := ""
	port := ""
	if strings.Contains(r.Host, ":") {
		host = strings.Split(r.Host, ":")[0]
		port = strings.Split(r.Host, ":")[1]
	} else {
		host = r.Host
		port = "9469"
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	if discoveryHost != "" {
		host = discoveryHost
	}
	if discoveryPort != "" {
		port = discoveryPort
	}
	if discoveryScheme != "" {
		scheme = discoveryScheme
	}

	var targets []target

	for _, script := range c.Scripts {
		labels := map[string]string{
			"__scheme__":       scheme,
			"__metrics_path__": path.Join(routePrefix, "/probe"),
			"__param_script":   script.Name,
		}
		if script.Discovery.ScrapeInterval != "" {
			labels["__scrape_interval__"] = script.Discovery.ScrapeInterval
		}
		if script.Discovery.ScrapeTimeout != "" {
			labels["__scrape_timeout__"] = script.Discovery.ScrapeTimeout
		}

		var params []string
		for key, value := range script.Discovery.Params {
			params = append(params, key)
			labels[fmt.Sprintf("__param_%s", key)] = value
		}
		labels["__param_params"] = strings.Join(params, ",")

		targets = append(targets, target{
			Targets: []string{fmt.Sprintf("%s:%s", host, port)},
			Labels:  labels,
		})
	}

	data, err := json.Marshal(targets)
	if err != nil {
		logger.Error("Failed to create discovery targets", slog.Any("error", err))
	}

	w.Write(data)
}
