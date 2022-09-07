package exporter

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ricoberger/script_exporter/pkg/auth"
	"github.com/ricoberger/script_exporter/pkg/config"
	"github.com/ricoberger/script_exporter/pkg/version"
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
}

//NewExporter return an exporter object with all its variables
func NewExporter(configFile string, createToken bool, timeoutOffset float64, noargs bool) (e *Exporter) {
	e = &Exporter{
		Config:        &config.Config{},
		timeoutOffset: timeoutOffset,
		noargs:        noargs,
		server:        &http.Server{},
	}

	// Load configuration file
	err := e.Config.LoadConfig(configFile)
	if err != nil {
		log.Fatalln(err)
	}

	// Validate configuration
	errs := config.ValidateConfig(e.Config)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Printf("Miconfiguration detected: %s", err)
		}
		log.Fatalln("Invalid configuration")
	}

	// Create bearer token
	if createToken {
		token, err := auth.CreateJWT(*e.Config)
		if err != nil {
			log.Fatalf("Bearer token could not be created: %s\n", err.Error())
		}

		fmt.Printf("Bearer token: %s\n", token)
		os.Exit(0)
	}

	return e
}

//InitExporter initialize the exporter, parse flags, init server, etc
func InitExporter() (e *Exporter) {

	listenAddress := flag.String("web.listen-address", ":9469", "Address to listen on for web interface and telemetry.")
	createToken := flag.Bool("create-token", false, "Create bearer token for authentication.")
	configFile := flag.String("config.file", "config.yaml", "Configuration `file` in YAML format.")
	timeoutOffset := flag.Float64("timeout-offset", 0.5, "Offset to subtract from Prometheus-supplied timeout in `seconds`.")
	noargs := flag.Bool("noargs", false, "Resctict script to accept arguments, for security issues")

	flag.Parse()

	// Avoid problems by erroring out if we have any remaining
	// arguments, instead of silently ignoring them.
	if len(flag.Args()) != 0 {
		log.Fatalf("Usage error: program takes no arguments, only options.")
	}

	e = NewExporter(*configFile, *createToken, *timeoutOffset, *noargs)

	// Start exporter
	fmt.Printf("Starting server %s\n", version.Info())
	fmt.Printf("Build context %s\n", version.BuildContext())
	fmt.Printf("script_exporter listening on %s\n", *listenAddress)

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
		if len(e.Config.Discovery.Host) > 0 {
			host = e.Config.Discovery.Host
		}
		if len(e.Config.Discovery.Port) > 0 {
			port = e.Config.Discovery.Port
		}
		if len(e.Config.Discovery.Scheme) > 0 {
			scheme = e.Config.Discovery.Scheme
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[ `))
		for idx, script := range e.Config.Scripts {
			w.Write([]byte(`{"targets": ["` + host + `:` + port + `"],`))
			w.Write([]byte(`"labels":{"__scheme__":"` + scheme + `","__metrics_path__":"/probe","__param_script":"` + script.Name + `"}}`))
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
		Handler: auth.Auth(router, *e.Config),
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

			err := e.server.Shutdown(ctx)
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
			err := e.Config.LoadConfig(*configFile)
			if err != nil {
				log.Printf("Could not reload configuration: %s\n", err.Error())
			} else {
				log.Printf("Configuration reloaded\n")
			}
		}
	}()

	return e
}

//Serve Start the http web server
func (e *Exporter) Serve() {
	if e.Config.TLS.Enabled {
		log.Fatalln(e.server.ListenAndServeTLS(e.Config.TLS.Crt, e.Config.TLS.Key))
	} else {
		log.Fatalln(e.server.ListenAndServe())
	}
}
