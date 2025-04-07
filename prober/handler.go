package prober

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ricoberger/script_exporter/config"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/common/expfmt"
)

type scriptResult struct {
	startTime time.Time
	success   int
	exitCode  int
	cached    int
	output    string
}

var (
	metricScriptUnknownTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "script_exporter",
		Name:      "script_unknown_total",
		Help:      "Total number of unknown scripts requested by probes",
	})
	metricReqInflight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "script_exporter",
		Name:      "http_requests_inflight",
		Help:      "Number of HTTP inflight requests, partitioned by script.",
	}, []string{"script"})
	metricReqCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "script_exporter",
		Name:      "http_requests_total",
		Help:      "Number of HTTP requests processed, partitioned by script.",
	}, []string{"script"})
	metricReqDurationSeconds = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  "script_exporter",
		Name:       "http_request_duration_seconds",
		Help:       "Latency of HTTP requests processed, partitioned by script.",
		Objectives: map[float64]float64{0.25: 0.05, 0.5: 0.05, 0.75: 0.02, 0.9: 0.01, 0.99: 0.001, 1.0: 0.001},
	}, []string{"script"})
)

func Handler(w http.ResponseWriter, r *http.Request, c *config.Config, logger *slog.Logger, logEnv bool, scriptTimeoutOffset float64, scriptNoArgs bool) {
	w.Header().Set("Content-Type", "text/plain")

	prometheusTimeout := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds")
	params := r.URL.Query()
	scriptNames := params["script"]

	if len(scriptNames) == 0 {
		logger.Error("'script' parameter is missing")
		metricScriptUnknownTotal.Inc()
		http.Error(w, "'script' parameter is missing", http.StatusBadRequest)
		return
	}

	for _, scriptName := range scriptNames {
		script := c.GetScript(scriptName)
		if script == nil {
			logger.Error("Script not found", "script", r.URL.Query().Get("script"))
			metricScriptUnknownTotal.Inc()
			http.Error(w, "Script not found", http.StatusBadRequest)
			return
		}

		metricReqInflight.WithLabelValues(scriptName).Inc()
		defer metricReqInflight.WithLabelValues(scriptName).Dec()

		start := time.Now()

		output := handleScript(script, params, logger, logEnv, prometheusTimeout, scriptTimeoutOffset, scriptNoArgs)

		logger.Debug("Script was run", slog.Duration("duration", time.Since(start)), slog.String("output", output))
		metricReqCount.WithLabelValues(scriptName).Inc()
		metricReqDurationSeconds.WithLabelValues(scriptName).Observe(time.Since(start).Seconds())
		fmt.Fprint(w, output)
	}
}

func handleScript(script *config.Script, params url.Values, logger *slog.Logger, logEnv bool, prometheusTimeout string, scriptTimeoutOffset float64, scriptNoArgs bool) string {
	// Get parameters, if the scriptNoArgs flag is set to true, we do not add
	// arguments from the params query parameter to the script.
	var scriptParamValues []string
	if !scriptNoArgs {
		scriptParams := params.Get("params")
		if scriptParams != "" {
			scriptParamValues = strings.Split(scriptParams, ",")

			for i, p := range scriptParamValues {
				scriptParamValues[i] = params.Get(p)
			}
		}
	}

	result := scriptResult{
		startTime: time.Now(),
		success:   1,
		exitCode:  -1,
		cached:    0,
		output:    "",
	}

	// Check if the result of the script is cached and not stale. If this is the
	// case the getCacheResult function will return a scriptResult which we can
	// directly return.
	if cachedResult := getCacheResult(script, scriptParamValues, false); cachedResult != nil {
		cachedResult.startTime = result.startTime
		cachedResult.cached = 1

		logger.Debug("Using cached script result", "script", script.Name)
		return generateScriptMetrics(script, *cachedResult)
	}

	// Get the timeout from either Prometheus's HTTP header or a URL query
	// parameter, clamped to a maximum specified through the configuration file.
	timeout := getTimeout(params, prometheusTimeout, scriptTimeoutOffset, script.Timeout.MaxTimeout)

	// Append arguments passed via scrape query parameters to the arguments
	// defined in the script configuration.
	runArgs := []string{}
	if script.Sudo {
		runArgs = append(runArgs, "sudo")
	}
	runArgs = append(runArgs, script.Command...)
	runArgs = append(runArgs, script.Args...)
	runArgs = append(runArgs, scriptParamValues...)

	// Get environment variables which should be set for the script from the
	// script configuration and the query parameters. If the allow_env_overwrite
	// option is set to true we overwrite environment variables from the script
	// configuration with the values from the query parameters.
	runEnv := make(map[string]string)
	for key, val := range script.Env {
		runEnv[key] = val
	}
	for key, val := range params {
		if _, ok := runEnv[key]; !ok || script.AllowEnvOverwrite {
			runEnv[key] = strings.Join(val, ",")
		}
	}

	output, exitCode, err := runScript(script, logger, logEnv, timeout, runArgs, runEnv)
	result.exitCode = exitCode
	result.output = getFormattedOutput(script, logger, output, err)

	if err != nil {
		result.success = 0

		if script.Cache.UseExpiredCacheOnError {
			if cachedResult := getCacheResult(script, scriptParamValues, true); cachedResult != nil {
				cachedResult.startTime = result.startTime
				cachedResult.cached = 1

				logger.Debug("Using cached script result", "script", script.Name)
				return generateScriptMetrics(script, *cachedResult)
			}
		}

		if script.Cache.CacheOnError {
			setCacheResult(script, scriptParamValues, result)
		}

		return generateScriptMetrics(script, result)
	}

	setCacheResult(script, scriptParamValues, result)
	return generateScriptMetrics(script, result)
}

func generateScriptMetrics(script *config.Script, result scriptResult) string {
	return `
# HELP script_success Script exit status (0 = error, 1 = success).
# TYPE script_success gauge
script_success{script="` + script.Name + `"} ` + fmt.Sprintf("%d", result.success) + `

# HELP script_duration_seconds Script execution time, in seconds.
# TYPE script_duration_seconds gauge
script_duration_seconds{script="` + script.Name + `"} ` + fmt.Sprintf("%f", time.Since(result.startTime).Seconds()) + `

# HELP script_exit_code The exit code of the script.
# TYPE script_exit_code gauge
script_exit_code{script="` + script.Name + `"} ` + fmt.Sprintf("%d", result.exitCode) + `

# HELP script_cached Script result is returned from cache (0 = no, 1 = yes).
# TYPE script_cached gauge
script_cached{script="` + script.Name + `"} ` + fmt.Sprintf("%d", result.cached) + `

` + result.output + `
`
}

// getTimeout gets the Prometheus scrape timeout (in seconds) from the HTTP
// request, either from a 'timeout' query parameter or from the special HTTP
// header that Prometheus inserts on scrapes, and returns it. If there is a
// timeout, it is modified down by the offset.
//
// If the there is an error or no timeout is specified, it returns the
// maxTimeout configured for the script (the default value for this is 0, which
// means no timeout)
func getTimeout(params url.Values, prometheusTimeout string, scriptTimeoutOffset float64, scriptMaxTimeout float64) float64 {
	v := params.Get("timeout")
	if v == "" {
		v = prometheusTimeout
	}
	if v == "" {
		return scriptMaxTimeout
	}
	ts, err := strconv.ParseFloat(v, 64)
	adjusted := ts - scriptTimeoutOffset
	switch {
	case err != nil:
		return scriptMaxTimeout
	case scriptMaxTimeout < adjusted && scriptMaxTimeout > 0:
		return scriptMaxTimeout
	case adjusted <= 0:
		return 0
	default:
		return adjusted
	}
}

func runScript(script *config.Script, logger *slog.Logger, logEnv bool, timeout float64, args []string, env map[string]string) (string, int, error) {
	// Tentatively, we do not inherit the context from the HTTP request. Doing
	// so would provide automatic termination should the client close the
	// connection, but it would mean that all scripts would be subject to abrupt
	// termination regardless of any 'enforced' settings. Right now, abrupt
	// termination requires opting in in the configuration file.
	var cancel context.CancelFunc
	ctx := context.Background()
	deadline := time.Now().Add(time.Duration(timeout * float64(time.Second)))

	if timeout > 0 && script.Timeout.Enforced {
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	//nolint:gosec
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	// When the executed script spawns it's own child processes (e.g. "sleep")
	// the child process will not be killed when the context deadline is
	// exceeded. Because "cmd.Wait()" waits for the command to exit and for
	// outputs being copied it will only return if the child process also
	// finishes. To enforce the timeout we can set "cmd.WaitDelay" to a non-zero
	// value to ensure that the executet script is killed even if the io pipes
	// are not closed.
	//
	// See:
	//   - https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773
	//   - https://stackoverflow.com/q/71714228
	if script.Timeout.WaitDelay > 0 {
		cmd.WaitDelay = time.Duration(script.Timeout.WaitDelay * float64(time.Second))
	}

	// Set environments variables
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// If the timeout larger than zero, it is exported into the environment as
	// $SCRIPT_TIMEOUT (its raw value) and $SCRIPT_DEADLINE, which is the Unix
	// timestamp (including fractional parts) when the deadline will expire. If
	// enforced is true, the timeout will be enforced by script_exporter, by
	// killing the script if the timeout is reached, and
	// $SCRIPT_TIMEOUT_ENFORCED will be set to 1 in the environment to inform
	// the script of this.
	if timeout > 0 {
		// Three digits of fractional precision in the seconds and the deadline
		// are probably excessive, given that we're running external programs.
		// But better slightly excessive than not enough precision.
		cmd.Env = append(cmd.Env, fmt.Sprintf("SCRIPT_TIMEOUT=%0.3f", timeout))
		cmd.Env = append(cmd.Env, fmt.Sprintf("SCRIPT_DEADLINE=%0.3f", float64(deadline.UnixNano())/float64(time.Second)))
		var envEnforced int
		if script.Timeout.Enforced {
			envEnforced = 1
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("SCRIPT_TIMEOUT_ENFORCED=%d", envEnforced))
	}

	logEnvValues := ""
	if logEnv {
		logEnvValues = strings.Join(cmd.Env, ",")
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			logger.Error("Script execution failed", slog.String("script", script.Name), slog.String("args", strings.Join(args, ",")), slog.String("env", logEnvValues), slog.String("stdout", stdout.String()), slog.String("stderr", stderr.String()), slog.Int("exitCode", exitError.ExitCode()), slog.Any("error", err))
			return stdout.String(), exitError.ExitCode(), err
		}

		logger.Error("Script execution failed", slog.String("script", script.Name), slog.String("args", strings.Join(args, ",")), slog.String("env", logEnvValues), slog.String("stdout", stdout.String()), slog.String("stderr", stderr.String()), slog.Int("exitCode", -1), slog.Any("error", err))
		return stdout.String(), -1, err
	}

	logger.Debug("Script execution succeeded", slog.String("script", script.Name), slog.String("args", strings.Join(args, ",")), slog.String("env", logEnvValues), slog.String("stdout", stdout.String()), slog.String("stderr", stderr.String()), slog.Int("exitCode", 0))
	return stdout.String(), 0, nil
}

func getFormattedOutput(script *config.Script, logger *slog.Logger, output string, err error) string {
	if script.Output.Ignore {
		return ""
	}

	if err != nil && script.Output.IgnoreOnError {
		return ""
	}

	var formattedOutput string
	var parser expfmt.TextParser

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		if len(scanner.Text()) == 0 {
			continue
		}

		_, err := parser.TextToMetricFamilies(strings.NewReader(fmt.Sprintf("%s\n", scanner.Text())))
		if err != nil {
			logger.Debug("Error parsing metric families", slog.String("script", script.Name), slog.String("output", scanner.Text()), slog.Any("error", err))
		} else {
			formattedOutput += fmt.Sprintf("%s\n", scanner.Text())
		}
	}

	return formattedOutput
}
