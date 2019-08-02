# script_exporter

The script_exporter is a [Prometheus](https://prometheus.io) exporter to execute scripts and collect metrics from the output or the exit status. The scripts to be executed are defined via a configuration file. In the configuration file several scripts can be specified. The script which should be executed is indicated by a parameter in the scrap configuration. The output of the script is captured and is provided for Prometheus. Even if the script does not produce any output, the exit status and the duration of the execution are provided.

## Building and running

Prerequisites:

- [Go compiler](https://golang.org/dl/)

Building:

```
git clone https://github.com/ricoberger/script_exporter.git
cd script_exporter

make build
```

Running:

```
./bin/script_exporter
```

Then visit [http://localhost:9469](http://localhost:9469) in the browser of your choice. There you have access to the following examples:

- [test](http://localhost:9469/probe?script=test&prefix=test): Invalid values which are returned by the script are omitted.
- [ping](http://localhost:9469/probe?script=ping&prefix=test&params=target&target=example.com): Pings the specified address in the `target` parameter and returns if it was successful or not.
- [helloworld](http://localhost:9469/probe?script=helloworld): Returns the specified argument in the `script` as label.
- [curltest](http://localhost:9469/probe?script=curltest&params=target&target=https://example.com): Runs a binary, which performs a get request against the specified `target` and returns the status code.
- [showtimeout](http://localhost:9469/probe?script=showtimeout&timeout=37): Reports whether or not the script is being run with a timeout from Prometheus, and what it is.
- [metrics](http://localhost:9469/metrics): Shows internal metrics from the script exporter.

## Usage and configuration

The script_exporter is configured via a configuration file and command-line flags.

```
Usage of ./bin/script_exporter:
  -config.file string
    	Configuration file in YAML format. (default "config.yaml")
  -create-token
    	Create bearer token for authentication.
  -timeout-offset seconds
        Offset to subtract from Prometheus-supplied timeout in seconds. (default 0.5)
  -version
    	Show version information.
  -web.listen-address string
    	Address to listen on for web interface and telemetry. (default ":9469")
```

The configuration file is written in YAML format, defined by the scheme described below.

```yaml
tls:
  active: <boolean>
  crt: <string>
  key: <string>

basicAuth:
  active: <boolean>
  username: <string>
  password: <string>

bearerAuth:
  active: <boolean>
  signingKey: <string>

timeouts:
  # in seconds, default 0 (no maximum)
  max_timeout: <float>
  enforced: <boolean>

scripts:
  - name: <string>
    script: <string>
    # optional
    timeout:
      max_timeout: <float>
      enforced: <boolean>
```

The `script` string will be split on spaces to generate the program name and any fixed arguments, then any arguments specified from the `params` parameter will be appended. The program will be executed directly, without a shell being invoked, and it is recommended that it be specified by path instead of relying on ``$PATH``.

Prometheus will normally provide an indication of its scrape timeout to the script exporter (through a special HTTP header). This information is made available to scripts through the environment variables `$SCRIPT_TIMEOUT` and `$SCRIPT_DEADLINE`. The first is the timeout in seconds (including a fractional part) and the second is the Unix timestamp when the deadline will expire (also including a fractional part). A simple script could implement this timeout by starting with `timeout "$SCRIPT_TIMEOUT" cmd ...`. A more sophisticated program might want to use the deadline time to compute internal timeouts for various operation. If `enforced` is true, either globally or for the script, `script_exporter` attempts to enforce the timeout by killing the script's main process after the timeout expires. The default is to not enforce timeouts.

For testing purposes, the timeout can be specified directly as a URL parameter (`timeout`). If present, the URL parameter takes priority over the Prometheus HTTP header. If `max_timeout` is set in the configuration file, it limits the maximum timeout value that HTTP requests can specify. A `max_timeout` of 0 means no limit, allowing a script to remove a general `max_timeout`.

## Prometheus configuration

The script_exporter needs to be passed the script name as a parameter (`script`). You can also pass a custom prefix (`prefix`) which is prepended to metrics names and the names of additional parameters which should be passed to the script (`params` and then additional URL parameters). If the `output` parameter is set to `ignore` then the script_exporter only return `script_success{}` and `script_duration_seconds{}`.

The `params` parameter is a comma-separated list of additional URL query parameters that will be used to construct the additional list of arguments, in order. The value of each URL query parameter is not parsed or split; it is passed directly to the script as a single argument.

Example config:

```yaml
scrape_configs:
  - job_name: 'script_test'
    metrics_path: /probe
    params:
      script: [test]
      prefix: [script]
    static_configs:
      - targets:
        - 127.0.0.1
    relabel_configs:
      - target_label: script
        replacement: test
  - job_name: 'script_ping'
    scrape_interval: 1m
    scrape_timeout: 30s
    metrics_path: /probe
    params:
      script: [ping]
      prefix: [script_ping]
      params: [target]
      output: [ignore]
    static_configs:
      - targets:
        - example.com
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - target_label: __address__
        replacement: 127.0.0.1:9469
      - source_labels: [__param_target]
        target_label: target
      - source_labels: [__param_target]
        target_label: instance

  - job_name: 'script_exporter'
    metrics_path: /metrics
    static_configs:
      - targets:
        - 127.0.0.1:9469
```

## Breaking changes

Changes from version 1.3.0:
- The command line flag ``-web.telemetry-path`` has been removed and its value is now always ``/probe``, which is a change from the previous default of ``/metrics``. The path ``/metrics`` now responds with Prometheus metrics for script_exporter itself.
- The command line flag ``-config.shell`` has been removed. Programs are now always run directly.

## Dependencies

- [yaml.v2 - YAML support for the Go language](gopkg.in/yaml.v2)
- [jwt-go - Golang implementation of JSON Web Tokens (JWT)](github.com/dgrijalva/jwt-go)
- [prometheus client_golang - Prometheus instrumentation library for Go applications](https://github.com/prometheus/client_golang/)
