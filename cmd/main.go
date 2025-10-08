package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/ricoberger/script_exporter/config"
	"github.com/ricoberger/script_exporter/discovery"
	"github.com/ricoberger/script_exporter/prober"

	"github.com/alecthomas/kingpin/v2"
	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

var (
	sc = config.NewSafeConfig(prometheus.DefaultRegisterer)

	configFiles          = kingpin.Flag("config.files", "Configuration files. To specify multiple configuration files glob patterns can be used.").Default("scripts.yaml").String()
	configURL            = kingpin.Flag("config.url", "URL to load configuration from (overrides --config.files).").Default("").String()
	configCheck          = kingpin.Flag("config.check", "If true, validate the configuration files and then exit.").Default().Bool()
	configReloadInterval = kingpin.Flag("config.reload-interval", "Interval for automatic config reload (e.g., 5m). If 0, no periodic reload.").Default("0").Duration()
	logEnv               = kingpin.Flag("log.env", "If true, environment variables passed to a script will be logged.").Default().Bool()
	scriptNoArgs         = kingpin.Flag("script.no-args", "Restrict script to accept arguments.").Default().Bool()
	scriptTimeoutOffset  = kingpin.Flag("script.timeout-offset", "Offset to subtract from timeout in seconds.").Default("0.5").Float64()
	externalURL          = kingpin.Flag("web.external-url", "The URL under which Script Exporter is externally reachable (for example, if Script Exporter is served via a reverse proxy). Used for generating relative and absolute links back to Script Exporter itself. If the URL has a path portion, it will be used to prefix all HTTP endpoints served by Script Exporter. If omitted, relevant URL components will be derived automatically.").PlaceHolder("<url>").String()
	routePrefix          = kingpin.Flag("web.route-prefix", "Prefix for the internal routes of web endpoints. Defaults to path of --web.external-url.").PlaceHolder("<path>").String()
	discoveryHost        = kingpin.Flag("discovery.host", "Host for service discovery.").Default("").String()
	discoveryPort        = kingpin.Flag("discovery.port", "Port for service discovery.").Default("").String()
	discoveryScheme      = kingpin.Flag("discovery.scheme", "Scheme for service discovery.").Default("").String()
	toolkitFlags         = webflag.AddFlags(kingpin.CommandLine, ":9469")
)

func init() {
	prometheus.MustRegister(versioncollector.NewCollector("script_exporter"))
}

func run(stopCh chan bool) int {
	kingpin.CommandLine.UsageWriter(os.Stdout)
	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print("script_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promslog.New(promslogConfig)

	logger.Info("Starting script_exporter", "version", version.Info())
	logger.Info(version.BuildContext())

	// Set config source based on flags
	if *configURL != "" {
		sc.ConfigSource = *configURL
		sc.IsURL = true
	} else {
		sc.ConfigSource = *configFiles
		sc.IsURL = false
	}

	if err := sc.ReloadConfig(logger); err != nil {
		logger.Error("Error loading config", "err", err)
		return 1
	}

	if *configCheck {
		logger.Info("Config is ok, exiting...")
		return 0
	}

	logger.Info("Loaded config")

	// Infer or set Script Exporter externalURL
	listenAddrs := toolkitFlags.WebListenAddresses
	if *externalURL == "" && *toolkitFlags.WebSystemdSocket {
		logger.Error("Cannot automatically infer external URL with systemd socket listener. Please provide --web.external-url")
		return 1
	} else if *externalURL == "" && len(*listenAddrs) > 1 {
		logger.Info("Inferring external URL from first provided listen address")
	}
	beURL, err := computeExternalURL(*externalURL, (*listenAddrs)[0])
	if err != nil {
		logger.Error("failed to determine external URL", "err", err)
		return 1
	}
	logger.Debug(beURL.String())

	// Default -web.route-prefix to path of -web.external-url.
	if *routePrefix == "" {
		*routePrefix = beURL.Path
	}

	// routePrefix must always be at least '/'.
	*routePrefix = "/" + strings.Trim(*routePrefix, "/")
	// routePrefix requires path to have trailing "/" in order for browsers to
	// interpret the path-relative path correctly, instead of stripping it.
	if *routePrefix != "/" {
		*routePrefix = *routePrefix + "/"
	}
	logger.Debug(*routePrefix)

	hup := make(chan os.Signal, 1)
	reloadCh := make(chan chan error)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-hup:
				if err := sc.ReloadConfig(logger); err != nil {
					logger.Error("Error reloading config", "err", err)
					continue
				}
				logger.Info("Reloaded config")
			case rc := <-reloadCh:
				if err := sc.ReloadConfig(logger); err != nil {
					logger.Error("Error reloading config", "err", err)
					rc <- err
				} else {
					logger.Info("Reloaded config")
					rc <- nil
				}
			}
		}
	}()

	// Match Prometheus behavior and redirect over externalURL for root path
	// only if routePrefix is different than "/".
	if *routePrefix != "/" {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, beURL.String(), http.StatusFound)
		})
	}

	http.HandleFunc(path.Join(*routePrefix, "/-/reload"),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				fmt.Fprintf(w, "This endpoint requires a POST request.\n")
				return
			}

			rc := make(chan error)
			reloadCh <- rc
			if err := <-rc; err != nil {
				http.Error(w, fmt.Sprintf("failed to reload config: %s", err), http.StatusInternalServerError)
			}
		})
	http.Handle(path.Join(*routePrefix, "/metrics"), promhttp.Handler())
	http.HandleFunc(path.Join(*routePrefix, "/-/healthy"), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Healthy"))
	})
	http.HandleFunc(path.Join(*routePrefix, "/probe"), func(w http.ResponseWriter, r *http.Request) {
		sc.Lock()
		config := sc.C
		sc.Unlock()
		prober.Handler(w, r, config, logger, *logEnv, *scriptTimeoutOffset, *scriptNoArgs)
	})
	http.HandleFunc(path.Join(*routePrefix, "/discovery"), func(w http.ResponseWriter, r *http.Request) {
		sc.Lock()
		config := sc.C
		sc.Unlock()
		discovery.Handler(w, r, config, logger, *discoveryHost, *discoveryPort, *discoveryScheme, *routePrefix)
	})
	http.HandleFunc(*routePrefix, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html>
		<head><title>Script Exporter</title></head>
		<body>
		<h1>Script Exporter</h1>
		<ul>
		<li><a href='/metrics'>Metrics</a></li>
		<li><a href='/probe'>Probe</a></li>
		<li><a href='/config'>Config</a></li>
		</ul>
		<ul>
		<li>version: ` + version.Version + `</li>
		<li>revision: ` + version.GetRevision() + `</li>
		<li>branch: ` + version.Branch + `</li>
		<li>buildUser: ` + version.BuildUser + `</li>
		<li>buildDate: ` + version.BuildDate + `</li>
		<li>goVersion: ` + version.GoVersion + `</li>
		<li>platform: ` + version.GoOS + `/` + version.GoArch + `</li>
		<li>tags: ` + version.GetTags() + `</li>
		</ul>
		</body>
		</html>`))
	})

	http.HandleFunc(path.Join(*routePrefix, "/config"), func(w http.ResponseWriter, r *http.Request) {
		sc.RLock()
		c, err := yaml.Marshal(sc.C)
		sc.RUnlock()
		if err != nil {
			logger.Warn("Error marshalling configuration", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write(c)
	})

	srv := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
	}
	srvc := make(chan struct{})
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := web.ListenAndServe(srv, toolkitFlags, logger); err != nil {
			logger.Error("Error starting HTTP server", "err", err)
			close(srvc)
		}
	}()

	// Periodic reload if interval is set
	if *configReloadInterval > 0 {
		ticker := time.NewTicker(*configReloadInterval)
		go func() {
			for range ticker.C {
				if err := sc.ReloadConfig(logger); err != nil {
					logger.Error("Periodic config check failed", "err", err)
				} else {
					logger.Debug("Periodic config check completed")
				}
			}
		}()
	}

	for {
		select {
		case <-stopCh:
			logger.Info("Service received stop message...")
			return 0
		case <-term:
			logger.Info("Received SIGTERM, exiting gracefully...")
			return 0
		case <-srvc:
			return 1
		}
	}
}

func startsOrEndsWithQuote(s string) bool {
	return strings.HasPrefix(s, "\"") || strings.HasPrefix(s, "'") ||
		strings.HasSuffix(s, "\"") || strings.HasSuffix(s, "'")
}

// computeExternalURL computes a sanitized external URL from a raw input. It
// infers unset URL parts from the OS and the given listen address.
func computeExternalURL(u, listenAddr string) (*url.URL, error) {
	if u == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		_, port, err := net.SplitHostPort(listenAddr)
		if err != nil {
			return nil, err
		}
		u = fmt.Sprintf("http://%s:%s/", hostname, port)
	}

	if startsOrEndsWithQuote(u) {
		return nil, errors.New("URL must not begin or end with quotes")
	}

	eu, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	ppref := strings.TrimRight(eu.Path, "/")
	if ppref != "" && !strings.HasPrefix(ppref, "/") {
		ppref = "/" + ppref
	}
	eu.Path = ppref

	return eu, nil
}
