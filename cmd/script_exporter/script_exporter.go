package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/ricoberger/script_exporter/pkg/config"
	"github.com/ricoberger/script_exporter/pkg/version"
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
	metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	showVersion   = flag.Bool("version", false, "Show version information.")
	createToken   = flag.Bool("create-token", false, "Create bearer token for authentication.")
	configFile    = flag.String("config.file", "config.yaml", "Configuration file in YAML format.")
	shell         = flag.String("config.shell", "", "Set shell to execute scripts with; otherwise they are executed directly")
)

func runScript(args []string) (string, error) {
	var output []byte
	var err error
	if *shell != "" {
		output, err = exec.Command(*shell, args...).Output()
	} else {
		output, err = exec.Command(args[0], args[1:]...).Output()
	}
	if err != nil {
		return "", err
	}

	return string(output), nil
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

	output, err := runScript(append(strings.Split(script, " "), paramValues...))
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
	regex1, _ := regexp.Compile("^" + prefix + "\\w*{.*}\\s+")
	regex2, _ := regexp.Compile("^" + prefix + "\\w*{.*}\\s+[0-9|\\.]*")

	var formatedOutput string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		metric := strings.Trim(scanner.Text(), " ")

		if metric == "" {
			// Do nothing
		} else if metric[0:1] == "#" {
			formatedOutput += fmt.Sprintf("%s\n", metric)
		} else {
			metric = fmt.Sprintf("%s%s", prefix, metric)
			metrics := regex1.FindAllString(metric, -1)
			if len(metrics) == 1 {
				value := strings.Replace(metric[len(metrics[0]):], ",", ".", -1)
				if regex2.MatchString(metrics[0] + value) {
					formatedOutput += fmt.Sprintf("%s%s\n", metrics[0], value)
				}
			}
		}
	}

	fmt.Fprintf(w, "%s\n%s\n%s_success{} %d\n%s\n%s\n%s_duration_seconds{} %f\n%s\n", scriptSuccessHelp, scriptSuccessType, namespace, 1, scriptDurationSecondsHelp, scriptDurationSecondsType, namespace, time.Since(scriptStartTime).Seconds(), formatedOutput)
	return
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

	http.HandleFunc(*metricsPath, use(metricsHandler, auth))
	http.HandleFunc("/", use(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		<head><title>Script Exporter</title></head>
		<body>
		<h1>Script Exporter</h1>
		<p><a href='` + *metricsPath + `'>Metrics</a></p>
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
	}, auth))

	if exporterConfig.TLS.Active == true {
		log.Fatalln(http.ListenAndServeTLS(*listenAddress, exporterConfig.TLS.Crt, exporterConfig.TLS.Key, nil))
	} else {
		log.Fatalln(http.ListenAndServe(*listenAddress, nil))
	}
}
