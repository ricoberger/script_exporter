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

Then visit [http://localhost:9469/metrics?script=test&prefix=test](http://localhost:9469/metrics?script=test&prefix=test) in the browser of your choice. There you should see the following output:

```
# HELP script_success Script exit status (0 = error, 1 = success).
# TYPE script_success gauge
script_success{} 1
# HELP script_duration_seconds Script execution time, in seconds.
# TYPE script_duration_seconds gauge
script_duration_seconds{} 0.006133
# HELP test_first_test
# TYPE test_first_test gauge
test_first_test{label="test_1_label_1"} 1
# HELP test_second_test
# TYPE test_second_test gauge
test_second_test{label="test_2_label_1",label="test_2_label_2"} 2.71828182846
# HELP test_third_test
# TYPE test_third_test gauge
test_third_test{} 3.14159265359
```

You can also visit the following url for a more complex example. The `ping` example uses the `params` parameter to check if a `target` is reachable: [http://localhost:9469/metrics?script=ping&prefix=test&params=target&target=example.com](http://localhost:9469/metrics?script=ping&prefix=test&params=target&target=example.com)

## Usage and configuration

The script_exporter is configured via a configuration file and command-line flags.

```
Usage of ./bin/script_exporter:
  -config.file string
    	Configuration file in YAML format. (default "config.yaml")
  -config.shell string
    	Set shell to execute scripts with; otherwise they are executed directly
  -create-token
    	Create bearer token for authentication.
  -version
    	Show version information.
  -web.listen-address string
    	Address to listen on for web interface and telemetry. (default ":9469")
  -web.telemetry-path string
    	Path under which to expose metrics. (default "/metrics")
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

scripts:
  - name: <string>
    script: <string>
```

The `script` string will be split on spaces to generate the program name and any fixed arguments, then any arguments specified from the `params` parameter will be appended. If a shell has not been set, the program will be executed directly; if a shell has been set, the shell will be used to run the script, executed as `SHELL script-name [argument ...]`. If a shell is set, what it runs must be a shell script (and for that shell); it cannot be a binary executable and any `#!` line at the start is ignored.

## Prometheus configuration

The script_exporter needs to be passed the script name as a parameter (`script`). You can also pass a custom prefix (`prefix`) which is prepended to metrics names and the names of additional parameters which should be passed to the script (`params` and then additional URL parameters). If the `output` parameter is set to `ignore` then the script_exporter only return `script_success{}` and `script_duration_seconds{}`.

The `params` parameter is a comma-separated list of additional URL query parameters that will be used to construct the additional list of arguments, in order. The value of each URL query parameter is not parsed or split; it is passed directly to the script as a single argument.

Example config:

```yaml
scrape_configs:
  - job_name: 'script_test'
    metrics_path: /metrics
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
    metrics_path: /metrics
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
```

## Dependencies

- [yaml.v2 - YAML support for the Go language](gopkg.in/yaml.v2)
- [jwt-go - Golang implementation of JSON Web Tokens (JWT)](github.com/dgrijalva/jwt-go)
