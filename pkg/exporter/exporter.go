package exporter

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	"github.com/prometheus/common/promlog"
	"github.com/ricoberger/script_exporter/pkg/auth"
	"github.com/ricoberger/script_exporter/pkg/config"
	customlog "github.com/ricoberger/script_exporter/pkg/log"
	"github.com/ricoberger/script_exporter/pkg/version"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace                 = "script"
	scriptSuccessHelp         = "# HELP script_success Script exit status (0 = error, 1 = success)."
	scriptSuccessType         = "# TYPE script_success gauge"
	scriptDurationSecondsHelp = "# HELP script_duration_seconds Script execution time, in seconds."
	scriptDurationSecondsType = "# TYPE script_duration_seconds gauge"
	scriptExitCodeHelp        = "# HELP script_exit_code The exit code of the script."
	scriptExitCodeType        = "# TYPE script_exit_code gauge"
)

type Exporter struct {
	Config        *config.Config
	timeoutOffset float64
	noargs        bool
	server        *http.Server
	Logger        log.Logger
}

// NewExporter return an exporter object with all its variables
func NewExporter(configFile string, createToken bool, timeoutOffset float64, noargs bool, logger log.Logger) (e *Exporter) {
	e = &Exporter{
		Config:        &config.Config{},
		timeoutOffset: timeoutOffset,
		noargs:        noargs,
		server:        &http.Server{},
		Logger:        logger,
	}

	// Load configuration file
	err := e.Config.LoadConfig(configFile)
	if err != nil {
		level.Error(logger).Log("err", err)
		os.Exit(1)
	}

	// Validate configuration
	errs := config.ValidateConfig(e.Config)
	if len(errs) > 0 {
		for _, err := range errs {
			level.Error(logger).Log("msg", "Miconfiguration detected", "err", err)
		}
		level.Error(logger).Log("err", "Invalid configuration")
		os.Exit(1)
	}

	// Create bearer token
	if createToken {
		token, err := auth.CreateJWT(*e.Config)
		if err != nil {
			level.Error(logger).Log("msg", "Bearer token could not be created", "err", err)
			os.Exit(1)
		}
		level.Info(logger).Log("msg", fmt.Sprintf("Bearer token: %s", token))
		os.Exit(0)
	}

	return e
}

// InitExporter initialize the exporter, parse flags, init server, etc
func InitExporter() (e *Exporter) {
	listenAddress := flag.String("web.listen-address", ":9469", "Address to listen on for web interface and telemetry.")
	createToken := flag.Bool("create-token", false, "Create bearer token for authentication.")
	configFile := flag.String("config.file", "config.yaml", "Configuration `file` in YAML format.")
	timeoutOffset := flag.Float64("timeout-offset", 0.5, "Offset to subtract from Prometheus-supplied timeout in `seconds`.")
	noargs := flag.Bool("noargs", false, "Restrict script to accept arguments, for security issues")
	logLevel := flag.String("log.level", "info", "Only log messages with the given severity or above. One of: [debug, info, warn, error]")
	logFormat := flag.String("log.format", "logfmt", "Output format of log messages. One of: [logfmt, json]")

	flag.Parse()

	allowedLevel := promlog.AllowedLevel{}
	allowedLevel.Set(*logLevel)

	allowedFormat := promlog.AllowedFormat{}
	allowedFormat.Set(*logFormat)

	promlogConfig := &promlog.Config{
		Level:  &allowedLevel,
		Format: &allowedFormat,
	}
	logger, err := customlog.InitLogger(promlogConfig)
	if err != nil {
		var logger log.Logger
		level.Error(logger).Log("msg", "Failed to init custom logger", "err", err)
		os.Exit(1)
	}

	// Avoid problems by erroring out if we have any remaining
	// arguments, instead of silently ignoring them.
	if len(flag.Args()) != 0 {
		level.Error(logger).Log("msg", "Usage error: program takes no arguments, only options.")
		os.Exit(1)
	}

	e = NewExporter(*configFile, *createToken, *timeoutOffset, *noargs, logger)

	// Start exporter
	level.Info(logger).Log("msg", "Starting script_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())
	level.Info(logger).Log("msg", fmt.Sprintf("Listening on %s", *listenAddress))

	// Authentication can be enabled via the 'basicAuth' or 'bearerAuth'
	// section in the configuration. If authentication is enabled it's
	// required for all routes.
	router := http.NewServeMux()

	router.Handle("/probe", SetupMetrics(e.MetricsHandler))
	router.Handle("/metrics", promhttp.Handler())
	router.HandleFunc("/discovery", func(w http.ResponseWriter, r *http.Request) {
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
		path   := ""
		if len(e.Config.Discovery.Host) > 0 {
			host = e.Config.Discovery.Host
		}
		if len(e.Config.Discovery.Port) > 0 {
			port = e.Config.Discovery.Port
		}
		if len(e.Config.Discovery.Scheme) > 0 {
			scheme = e.Config.Discovery.Scheme
		}
		if len(e.Config.Discovery.Path) > 0 {
			path = e.Config.Discovery.Path
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[ `))
		for idx, script := range e.Config.Scripts {
			w.Write([]byte(`{"targets": ["` + host + `:` + port + `"],`))
			w.Write([]byte(`"labels":{"__scheme__":"` + scheme + `","__metrics_path__":"` + path + `/probe","__param_script":"` + script.Name + `"}}`))
			if idx+1 < len(e.Config.Scripts) {
				w.Write([]byte(`,`))
			}
		}
		w.Write([]byte(`]`))
	})
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

	e.server = &http.Server{
		Addr:    *listenAddress,
		Handler: auth.Auth(router, *e.Config, logger),
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

			e.server.Shutdown(ctx)
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
			err := e.Config.LoadConfig(*configFile)
			if err != nil {
				level.Error(logger).Log("msg", "Could not reload configuration", "err", err)
			} else {
				level.Info(logger).Log("msg", "Configuration reloaded")
			}
		}
	}()

	return e
}

// Serve Start the http web server
func (e *Exporter) Serve() {
	var err error
	if e.Config.TLS.Enabled {
		err = e.server.ListenAndServeTLS(e.Config.TLS.Crt, e.Config.TLS.Key)
	} else {
		err = e.server.ListenAndServe()
	}

	if err == http.ErrServerClosed {
		level.Info(e.Logger).Log("msg", "Shutdown script_exporter gracefully")
		os.Exit(0)
	} else {
		level.Error(e.Logger).Log("msg", "Failed to shutdown script_exporter gracefully", "err", err)
		os.Exit(1)
	}
}
