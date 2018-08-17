# script_exporter

The script exporter is a [Prometheus](https://prometheus.io) exporter to execute scripts and collect metrics from the output or the exit status. The scripts to be executed are defined via a configuration file. In the configuration file several scripts can be specified. The script which should be executed is indicated by a parameter in the scrap configuration. The output of the script is captured and is provided for Prometheus. Even if the script does not produce any output, the exit status and the duration of the execution are provided.

## Building and running

Prerequisites:

- [Go compiler](https://golang.org/dl/)

Building:

```
git clone https://github.com/ricoberger/script_exporter.git $GOPATH/src/github.com/ricoberger/script_exporter
cd $GOPATH/src/github.com/ricoberger/script_exporter

dep ensure
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

The script exporter is configured via a configuration file and command-line flags.

```
Usage of ./bin/script_exporter:
  -config.file string
    	Configuration file in YAML format. (default "config.yaml")
  -config.shell string
    	Shell to execute script (default "/bin/sh")
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

## Prometheus configuration

The script exporter needs to be passed the script name as a parameter (`script`). You can also pass a custom prefix (`prefix`) and additional parameters which should be passed to the script (`params`).

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
