// Script_exporter is a Prometheus exporter to execute programs and
// scripts and collect metrics from their output and their exit
// status.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"bytes"

	"github.com/ricoberger/script_exporter/pkg/config"
	"github.com/ricoberger/script_exporter/pkg/version"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace                 = "script"
	scriptSuccessHelp         = "# HELP script_success Script exit status (0 = error, 1 = success)."
	scriptSuccessType         = "# TYPE script_success gauge"
	scriptDurationSecondsHelp = "# HELP script_duration_seconds Script execution time, in seconds."
	scriptDurationSecondsType = "# TYPE script_duration_seconds gauge"
)

var (
	exporterConfig config.Config

	listenAddress = flag.String("web.listen-address", ":9469", "Address to listen on for web interface and telemetry.")
	showVersion   = flag.Bool("version", false, "Show version information.")
	createToken   = flag.Bool("create-token", false, "Create bearer token for authentication.")
	configFile    = flag.String("config.file", "config.yaml", "Configuration `file` in YAML format.")
	timeoutOffset = flag.Float64("timeout-offset", 0.5, "Offset to subtract from Prometheus-supplied timeout in `seconds`.")
)

// runScript runs a program with some arguments; the program is
// args[0]. The timeout argument is in seconds, and if it's larger
// than zero, it is exported into the environment as $SCRIPT_TIMEOUT
// (its raw value) and $SCRIPT_DEADLINE, which is the Unix timestamp
// (including fractional parts) when the deadline will expire. If
// enforced is true, the timeout will be enforced by script_exporter,
// by killing the script if the timeout is reached, and
// $SCRIPT_TIMEOUT_ENFORCED will be set to 1 in the environment to
// inform the script of this.
//
// Note that killing the script is only a best effort attempt to
// terminate its execution and time out the request. Sub-processes may
// not be terminated, and termination may not be entirely successful.
//
// Tentatively, we do not inherit the context from the HTTP request.
// Doing so would provide automatic termination should the client
// close the connection, but it would mean that all scripts would
// be subject to abrupt termination regardless of any 'enforced:'
// settings. Right now, abrupt termination requires opting in in
// the configuration file.
func runScript(timeout float64, enforced bool, args []string) (string, error) {
	var output []byte
	var err error

	// We go through a great deal of work to get a deadline with
	// fractional seconds that we can expose in an environment
	// variable. However, this is pretty much necessary since
	// we've copied Blackbox's default of a half second adjustment
	// to the raw Prometheus timeout.  We can hardly do that and
	// then round our deadlines (or our raw timeouts) off to full
	// seconds.
	ns := float64(time.Second)
	deadline := time.Now().Add(time.Duration(timeout * ns))
	dlfractional := float64(deadline.UnixNano()) / ns

	var cmd *exec.Cmd
	var cancel context.CancelFunc
	ctx := context.Background()
	if timeout > 0 && enforced {
		ctx, cancel = context.WithDeadline(context.Background(), deadline)
		defer cancel()
	}
	cmd = exec.CommandContext(ctx, args[0], args[1:]...)

	if timeout > 0 {
		cmd.Env = os.Environ()
		// Three digits of fractional precision in the seconds and
		// the deadline are probably excessive, given that we're
		// running external programs. But better slightly excessive
		// than not enough precision.
		cmd.Env = append(cmd.Env, fmt.Sprintf("SCRIPT_TIMEOUT=%0.3f", timeout))
		cmd.Env = append(cmd.Env, fmt.Sprintf("SCRIPT_DEADLINE=%0.3f", dlfractional))
		var ienforced int
		if enforced {
			ienforced = 1
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("SCRIPT_TIMEOUT_ENFORCED=%d", ienforced))
	}

	output, err = cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// getTimeout gets the Prometheus scrape timeout (in seconds) from the
// HTTP request, either from a 'timeout' query parameter or from the
// special HTTP header that Prometheus inserts on scrapes, and returns
// it or 0 on error.  If there is a timeout, it is modified down by
// the offset.
func getTimeout(r *http.Request, offset float64, maxTimeout float64) float64 {
	v := r.URL.Query().Get("timeout")
	if v == "" {
		v = r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds")
	}
	if v == "" {
		return 0
	}
	ts, err := strconv.ParseFloat(v, 64)
	adjusted := ts - offset
	switch {
	case err != nil:
		return 0
	case maxTimeout < adjusted && maxTimeout > 0:
		return maxTimeout
	case adjusted <= 0:
		return 0
	default:
		return adjusted
	}
}

// instrumentScript wraps the underlying http.Handler with Prometheus
// instrumentation to produce per-script metrics on the number of
// requests in flight, the number of requests in total, and the
// distribution of their duration. Requests without a 'script=' query
// parameter are not instrumented (and will probably be rejected).
func instrumentScript(obs prometheus.ObserverVec, cnt *prometheus.CounterVec, g *prometheus.GaugeVec, next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sn := r.URL.Query().Get("script")
		if sn == "" {
			// Rather than make up a fake script label, such
			// as "NONE", we let the request fall through without
			// instrumenting it. Under normal circumstances it
			// will fail anyway, as metricsHandler() will
			// reject it.
			next.ServeHTTP(w, r)
			return
		}

		labels := prometheus.Labels{"script": sn}
		g.With(labels).Inc()
		defer g.With(labels).Dec()
		now := time.Now()
		next.ServeHTTP(w, r)
		obs.With(labels).Observe(time.Since(now).Seconds())
		cnt.With(labels).Inc()
	})
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Get script from url parameter
	params := r.URL.Query()
	scriptName := params.Get("script")
	if scriptName == "" {
		log.Printf("Script parameter is missing\n")
		http.Error(w, "Script parameter is missing", http.StatusBadRequest)
		return
	}

	// Get prefix from url parameter
	prefix := params.Get("prefix")
	if prefix != "" {
		prefix = fmt.Sprintf("%s_", prefix)
	}

	// Get parameters
	var paramValues []string
	scriptParams := params.Get("params")
	if scriptParams != "" {
		paramValues = strings.Split(scriptParams, ",")

		for i, p := range paramValues {
			paramValues[i] = params.Get(p)
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	scriptStartTime := time.Now()

	// Get and run script
	script := exporterConfig.GetScript(scriptName)
	if script == "" {
		log.Printf("Script not found\n")
		http.Error(w, "Script not found", http.StatusBadRequest)
		return
	}

	// Get the timeout from either Prometheus's HTTP header or a URL
	// query parameter, clamped to a maximum specified through the
	// configuration file.
	timeout := getTimeout(r, *timeoutOffset, exporterConfig.GetMaxTimeout(scriptName))

	output, err := runScript(timeout, exporterConfig.GetTimeoutEnforced(scriptName), append(strings.Split(script, " "), paramValues...))
	if err != nil {
		log.Printf("Script failed: %s\n", err.Error())
		fmt.Fprintf(w, "%s\n%s\n%s_success{} %d\n%s\n%s\n%s_duration_seconds{} %f\n", scriptSuccessHelp, scriptSuccessType, namespace, 0, scriptDurationSecondsHelp, scriptDurationSecondsType, namespace, time.Since(scriptStartTime).Seconds())
		return
	}

	// Get ignore output parameter and only return success and duration seconds if 'true'
	outputParam := params.Get("output")
	if outputParam == "ignore" {
		fmt.Fprintf(w, "%s\n%s\n%s_success{} %d\n%s\n%s\n%s_duration_seconds{} %f\n", scriptSuccessHelp, scriptSuccessType, namespace, 1, scriptDurationSecondsHelp, scriptDurationSecondsType, namespace, time.Since(scriptStartTime).Seconds())
		return
	}

	// Format output
	regex1, _ := regexp.Compile("^" + prefix + "\\w*(?:{.*})?\\s+")
	regex2, _ := regexp.Compile("^" + prefix + "\\w*(?:{.*})?\\s+[0-9|\\.]*")
	regexSharp, _ := regexp.Compile("^(# *(?:TYPE|HELP) +)")
	regexSharpReplace := "${1}" + prefix

	var formatedBuffer bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		metric := strings.Trim(scanner.Text(), " ")

		if metric == "" {
			// Do nothing
		} else if metric[0:1] == "#" {
			if prefix != "" {
				metric = regexSharp.ReplaceAllString(metric, regexSharpReplace)
			}
			formatedBuffer.WriteString(metric)
			formatedBuffer.WriteString("\n")
		} else {
			metric = prefix + metric
			metrics := regex1.FindAllString(metric, -1)
			if len(metrics) == 1 {
				value := strings.Replace(metric[len(metrics[0]):], ",", ".", -1)
				if regex2.MatchString(metrics[0] + value) {
					formatedBuffer.WriteString(metrics[0])
					formatedBuffer.WriteString(value)
					formatedBuffer.WriteString("\n")
				}
			}
		}
	}

	fmt.Fprintf(w, "%s\n%s\n%s_success{} %d\n%s\n%s\n%s_duration_seconds{} %f\n%s\n", scriptSuccessHelp, scriptSuccessType, namespace, 1, scriptDurationSecondsHelp, scriptDurationSecondsType, namespace, time.Since(scriptStartTime).Seconds(), formatedBuffer.String())
}

// setupMetrics creates and registers our internal Prometheus metrics,
// and then wraps up a http.HandlerFunc into a http.Handler that
// properly counts all of the metrics when a request happens.
//
// Portions of it are taken from the promhttp examples.
//
// We use the 'scripts' namespace for our internal metrics so that
// they don't collide with the 'script' namespace for probe results.
func setupMetrics(h http.HandlerFunc) http.Handler {
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

func main() {
	// Parse command-line flags
	flag.Parse()

	// Show version information
	if *showVersion {
		v, err := version.Print("script_exporter")
		if err != nil {
			log.Fatalf("Failed to print version information: %#v", err)
		}

		fmt.Fprintln(os.Stdout, v)
		os.Exit(0)
	}

	// Avoid problems by erroring out if we have any remaining
	// arguments, instead of silently ignoring them.
	if len(flag.Args()) != 0 {
		log.Fatalf("Usage error: program takes no arguments, only options.")
	}

	// Load configuration file
	err := exporterConfig.LoadConfig(*configFile)
	if err != nil {
		log.Fatalln(err)
	}

	// Create bearer token
	if *createToken {
		token, err := createJWT()
		if err != nil {
			log.Fatalf("Bearer token could not be created: %s\n", err.Error())
		}

		fmt.Printf("Bearer token: %s\n", token)
		os.Exit(0)
	}

	// Start exporter
	fmt.Printf("Starting server %s\n", version.Info())
	fmt.Printf("Build context %s\n", version.BuildContext())
	fmt.Printf("script_exporter listening on %s\n", *listenAddress)

	// Authentication can be enabled via the 'basicAuth' or 'bearerAuth'
	// section in the configuration. If authentication is enabled it's
	// required for all routes.
	router := http.NewServeMux()

	router.Handle("/probe", setupMetrics(metricsHandler))
	router.Handle("/metrics", promhttp.Handler())
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		<head><title>Script Exporter</title></head>
		<body>
		<h1>Script Exporter</h1>
		<p><a href='/metrics'>Metrics</a></p>
		<p><a href='/probe'>Probe</a></p>
		<p><ul>
		<li>version: ` + version.Version + `</li>
		<li>branch: ` + version.Branch + `</li>
		<li>revision: ` + version.Revision + `</li>
		<li>go version: ` + version.GoVersion + `</li>
		<li>build user: ` + version.BuildUser + `</li>
		<li>build date: ` + version.BuildDate + `</li>
		</ul></p>
		</body>
		</html>`))
	})

	server := &http.Server{
		Addr:    *listenAddress,
		Handler: auth(router),
	}

	// Listen for SIGINT and SIGTERM signals and try to gracefully shutdown
	// the HTTP server. This ensures that enabled connections are not
	// interrupted.
	go func() {
		term := make(chan os.Signal, 1)
		signal.Notify(term, os.Interrupt, syscall.SIGTERM)
		select {
		case <-term:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := server.Shutdown(ctx)
			if err != nil {
				log.Printf("Failed to shutdown script_exporter gracefully: %s\n", err.Error())
				os.Exit(1)
			}

			log.Printf("Shutdown script_exporter...\n")
			os.Exit(0)
		}
	}()

	// Listen for SIGHUP signal and reload the configuration. If the
	// configuration could not be reloaded, the old config will continue to be
	// used.
	go func() {
		hup := make(chan os.Signal, 1)
		signal.Notify(hup, syscall.SIGHUP)
		select {
		case <-hup:
			err := exporterConfig.LoadConfig(*configFile)
			if err != nil {
				log.Printf("Could not reload configuration: %s\n", err.Error())
			} else {
				log.Printf("Configuration reloaded\n")
			}
		}
	}()

	if exporterConfig.TLS.Enabled {
		log.Fatalln(server.ListenAndServeTLS(exporterConfig.TLS.Crt, exporterConfig.TLS.Key))
	} else {
		log.Fatalln(server.ListenAndServe())
	}
}
