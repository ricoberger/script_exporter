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

Then visit [http://localhost:9469/metrics?script=test](http://localhost:9469/metrics?script=test) in the browser of your choice. There you should see the following output:

```
test_success{} 1
test_duration_seconds{} 0.005626
# First test
test_first_test{label="test_1_label_1"} 1
# Second test
test_second_test{label="test_2_label_1",label="test_2_label_2"} 2.71828182846
# Third test
test_third_test{} 3.14159265359
```

## Usage and configuration

The script exporter is configured via a configuration file and command-line flags.

```
Usage of ./bin/script_exporter:
  -config.file string
    	Configuration file in YAML format. (default "config.yaml")
  -config.shell string
    	Shell to execute script (default "/bin/sh")
  -version
    	Show version information.
  -web.listen-address string
    	Address to listen on for web interface and telemetry. (default ":9469")
  -web.telemetry-path string
    	Path under which to expose metrics. (default "/metrics")
```

The configuration file is written in YAML format, defined by the scheme described below.

```yaml
scripts:
  [ - <script_config> ... ]

# script_config
name: <string>
script: <string>
```

## Prometheus configuration

The script exporter needs to be passed the script name as a parameter (`script`). You can also pass a custom prefix. If the `prefix` parameter is missing `script` will be used as prefix.

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
```

## Dependencies

- [yaml.v2 - YAML support for the Go language](gopkg.in/yaml.v2)
