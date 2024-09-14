package exporter

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ricoberger/script_exporter/pkg/version"

	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (e *Exporter) metricsHandler(scriptName string, params url.Values, prometheusTimeout string) (string, error) {
	// Get prefix from url parameter
	prefix := params.Get("prefix")
	if prefix != "" {
		prefix = fmt.Sprintf("%s_", prefix)
	}

	// Get parameters
	var paramValues []string
	if !e.noargs {
		scriptParams := params.Get("params")
		if scriptParams != "" {
			paramValues = strings.Split(scriptParams, ",")

			for i, p := range paramValues {
				paramValues[i] = params.Get(p)
			}
		}
	}

	scriptStartTime := time.Now()

	// When the script configuration contains a cache duration we use the result from the cache when the entry is not
	// stale.
	cacheDuration := e.Config.GetCacheDuration(scriptName)
	if cacheDuration != nil {
		formattedOutput, successStatus, exitCode := getCacheResult(scriptName, paramValues, *cacheDuration, false)
		if formattedOutput != nil && successStatus != nil && exitCode != nil {
			level.Debug(e.Logger).Log("msg", "Returning cached result", "script", scriptName)
			return fmt.Sprintf("%s\n%s\n%s_success{script=\"%s\"} %d\n%s\n%s\n%s_duration_seconds{script=\"%s\"} %f\n%s\n%s\n%s_exit_code{script=\"%s\"} %d\n%s\n%s\n%s_use_cache{script=\"%s\"} %d\n%s\n%s\n%s_use_expired_cache{script=\"%s\"} %d\n%s\n", scriptSuccessHelp, scriptSuccessType, namespace, scriptName, *successStatus, scriptDurationSecondsHelp, scriptDurationSecondsType, namespace, scriptName, time.Since(scriptStartTime).Seconds(), scriptExitCodeHelp, scriptExitCodeType, namespace, scriptName, *exitCode, scriptCacheHelp, scriptCacheType, namespace, scriptName, 1, scriptExpCacheHelp, scriptExpCacheType, namespace, scriptName, 0, *formattedOutput), nil
		}
	}

	// Get program name and static arguments
	runArgs, err := e.Config.GetRunArgs(scriptName)
	if err != nil {
		errorStr := fmt.Sprintf("Script '%s' not found", scriptName)
		level.Error(e.Logger).Log("err", errorStr, "script", scriptName)
		return "", fmt.Errorf("%s", errorStr)
	}
	// Append args passed via scrape query parameters
	runArgs = append(runArgs, paramValues...)

	// Get the timeout from either Prometheus's HTTP header or a URL
	// query parameter, clamped to a maximum specified through the
	// configuration file.
	timeout := getTimeout(params, prometheusTimeout, e.timeoutOffset, e.Config.GetMaxTimeout(scriptName))

	// Get env vars
	runEnv := e.Config.GetRunEnv(scriptName)

	envOverwriteAllowed := e.Config.GetAllowEnvOverwrite(scriptName)

	for key, paramValues := range params {
		if _, ok := runEnv[key]; !ok || envOverwriteAllowed {
			runEnv[key] = strings.Join(paramValues, ",")
		}
	}

	// Success status of the executed script
	successStatus := 1

	output, exitCode, err := runScript(scriptName, e.Logger, e.logEnv, timeout, e.Config.GetTimeoutEnforced(scriptName), runArgs, runEnv)
	if err != nil {
		successStatus = 0

		useExpiredCacheOnError := e.Config.GetUseExpiredCacheOnError(scriptName)
		if cacheDuration != nil && useExpiredCacheOnError {
			formattedOutput, successStatus, exitCode := getCacheResult(scriptName, paramValues, *cacheDuration, useExpiredCacheOnError)
			if formattedOutput != nil && successStatus != nil && exitCode != nil {
				level.Debug(e.Logger).Log("msg", "Returning expired cache result", "script", scriptName)
				return fmt.Sprintf("%s\n%s\n%s_success{script=\"%s\"} %d\n%s\n%s\n%s_duration_seconds{script=\"%s\"} %f\n%s\n%s\n%s_exit_code{script=\"%s\"} %d\n%s\n%s\n%s_use_cache{script=\"%s\"} %d\n%s\n%s\n%s_use_expired_cache{script=\"%s\"} %d\n%s\n", scriptSuccessHelp, scriptSuccessType, namespace, scriptName, *successStatus, scriptDurationSecondsHelp, scriptDurationSecondsType, namespace, scriptName, time.Since(scriptStartTime).Seconds(), scriptExitCodeHelp, scriptExitCodeType, namespace, scriptName, *exitCode, scriptCacheHelp, scriptCacheType, namespace, scriptName, 1, scriptExpCacheHelp, scriptExpCacheType, namespace, scriptName, 1, *formattedOutput), nil
			}
		}
	}

	// Get ignore output parameter and only return success and duration seconds if 'output=ignore'. If the script failed
	// we also have to check the ignoreOutputOnFail setting of the script to only return the output when it is set to
	// true.
	outputParam := params.Get("output")
	if outputParam == "ignore" || (successStatus == 0 && e.Config.GetIgnoreOutputOnFail(scriptName)) {
		return fmt.Sprintf("%s\n%s\n%s_success{script=\"%s\"} %d\n%s\n%s\n%s_duration_seconds{script=\"%s\"} %f\n%s\n%s\n%s_exit_code{script=\"%s\"} %d\n%s\n%s\n%s_use_cache{script=\"%s\"} %d\n%s\n%s\n%s_use_expired_cache{script=\"%s\"} %d\n", scriptSuccessHelp, scriptSuccessType, namespace, scriptName, successStatus, scriptDurationSecondsHelp, scriptDurationSecondsType, namespace, scriptName, time.Since(scriptStartTime).Seconds(), scriptExitCodeHelp, scriptExitCodeType, namespace, scriptName, exitCode, scriptCacheHelp, scriptCacheType, namespace, scriptName, 0, scriptExpCacheHelp, scriptExpCacheType, namespace, scriptName, 0), nil
	}

	// Format output
	regex1, _ := regexp.Compile("^" + prefix + "\\w*(?:{.*})?\\s+")
	regex2, _ := regexp.Compile("^" + prefix + "\\w*(?:{.*})?\\s+[0-9|\\.]*")
	regexSharp, _ := regexp.Compile("^(# *(?:TYPE|HELP) +)")

	var formattedOutput string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		metric := strings.Trim(scanner.Text(), " ")

		if metric == "" {
			// Do nothing
		} else if metric[0:1] == "#" {
			if prefix != "" {
				formattedOutput += regexSharp.ReplaceAllString(metric, "${1}"+prefix) + "\n"
			} else {
				formattedOutput += fmt.Sprintf("%s\n", metric)
			}
		} else {
			metric = fmt.Sprintf("%s%s", prefix, metric)
			metrics := regex1.FindAllString(metric, -1)
			if len(metrics) == 1 {
				value := strings.Replace(metric[len(metrics[0]):], ",", ".", -1)
				if regex2.MatchString(metrics[0] + value) {
					formattedOutput += fmt.Sprintf("%s%s\n", metrics[0], value)
				}
			}
		}
	}

	// If the script configuration has a cache duration we have to cache the output and exit code, so we can reuse it
	// later.
	if cacheDuration != nil {
		level.Debug(e.Logger).Log("msg", "Saving result to cache", "script", scriptName)
		setCacheResult(scriptName, paramValues, formattedOutput, successStatus, exitCode)
	}

	return fmt.Sprintf("%s\n%s\n%s_success{script=\"%s\"} %d\n%s\n%s\n%s_duration_seconds{script=\"%s\"} %f\n%s\n%s\n%s_exit_code{script=\"%s\"} %d\n%s\n%s\n%s_use_cache{script=\"%s\"} %d\n%s\n%s\n%s_use_expired_cache{script=\"%s\"} %d\n%s\n", scriptSuccessHelp, scriptSuccessType, namespace, scriptName, successStatus, scriptDurationSecondsHelp, scriptDurationSecondsType, namespace, scriptName, time.Since(scriptStartTime).Seconds(), scriptExitCodeHelp, scriptExitCodeType, namespace, scriptName, exitCode, scriptCacheHelp, scriptCacheType, namespace, scriptName, 0, scriptExpCacheHelp, scriptExpCacheType, namespace, scriptName, 0, formattedOutput), nil
}

func (e *Exporter) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	// Get Prometheus timeout header
	prometheusTimeout := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds")

	// Get scripts from url parameter
	params := r.URL.Query()
	scriptNames := params["script"]
	if len(scriptNames) == 0 {
		errorStr := "Script parameter is missing"
		level.Error(e.Logger).Log("err", errorStr)
		http.Error(w, errorStr, http.StatusBadRequest)
		return
	}

	// Run each script and return the output
	for _, scriptName := range scriptNames {
		output, err := e.metricsHandler(scriptName, params, prometheusTimeout)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, output)
	}
}

// SetupMetrics creates and registers our internal Prometheus metrics,
// and then wraps up a http.HandlerFunc into a http.Handler that
// properly counts all of the metrics when a request happens.
//
// Portions of it are taken from the promhttp examples.
//
// We use the 'scripts' namespace for our internal metrics so that
// they don't collide with the 'script' namespace for probe results.
func SetupMetrics(h http.HandlerFunc) http.Handler {
	// Broad metrics provided by promhttp, namespaced into
	// 'http' to make what they're about clear from their
	// names.
	reqs := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "http",
			Name:      "requests_total",
			Help:      "Total requests for scripts by HTTP result code and method.",
		},
		[]string{"code", "method"})
	rdur := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  "http",
			Name:       "requests_duration_seconds",
			Help:       "A summary of request durations by HTTP result code and method.",
			Objectives: map[float64]float64{0.25: 0.05, 0.5: 0.05, 0.75: 0.02, 0.9: 0.01, 0.99: 0.001, 1.0: 0.001},
		},
		[]string{"code", "method"})

	// Our per-script metrics, counting requests in flight and
	// requests total, and providing a time distribution.
	sreqs := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "scripts",
			Name:      "requests_total",
			Help:      "Total requests to a script",
		},
		[]string{"script"})
	sif := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "scripts",
			Name:      "requests_inflight",
			Help:      "Number of requests in flight to a script",
		},
		[]string{"script"})
	sdur := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  "scripts",
			Name:       "duration_seconds",
			Help:       "A summary of request durations to a script",
			Objectives: map[float64]float64{0.25: 0.05, 0.5: 0.05, 0.75: 0.02, 0.9: 0.01, 0.99: 0.001, 1.0: 0.001},
			//Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"script"},
	)

	// We also publish build information through a metric.
	buildInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "scripts",
			Name:      "build_info",
			Help:      "A metric with a constant '1' value labeled by build information.",
		},
		[]string{"version", "revision", "branch", "goversion", "builddate", "builduser"},
	)
	buildInfo.WithLabelValues(version.Version, version.Revision, version.Branch, version.GoVersion, version.BuildDate, version.BuildUser).Set(1)

	prometheus.MustRegister(rdur, reqs, sreqs, sif, sdur, buildInfo)

	// We don't use InstrumentHandlerInFlight, because that
	// duplicates what we're doing on a per-script basis. The
	// other promhttp handlers don't duplicate this work, because
	// they capture result code and method. This is slightly
	// questionable, but there you go.
	return promhttp.InstrumentHandlerDuration(rdur,
		promhttp.InstrumentHandlerCounter(reqs,
			instrumentScript(sdur, sreqs, sif, h)))
}
