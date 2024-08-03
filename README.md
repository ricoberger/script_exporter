# script_exporter

The script_exporter is a [Prometheus](https://prometheus.io) exporter to execute scripts and collect metrics from the output or the exit status. The scripts to be executed are defined via a configuration file. In the configuration file several scripts can be specified. The script which should be executed is indicated by a parameter in the scrap configuration. The output of the script is captured and is provided for Prometheus. Even if the script does not produce any output, the exit status and the duration of the execution are provided.

## Building and running

To run the script_exporter you can use the one of the binaries from the [release](https://github.com/ricoberger/script_exporter/releases) page or the [Docker image](https://hub.docker.com/r/ricoberger/script_exporter). You can also build the script_exporter by yourself by running the following commands:

```sh
git clone https://github.com/ricoberger/script_exporter.git
cd script_exporter
make build
```

An example configuration can be found in the [examples](./examples) folder. To use this configuration run the following command:

```sh
./bin/script_exporter -config.file ./examples/config.yaml
```

To run the examples via Docker the following commands can be used:

```sh
docker build -f ./Dockerfile -t ghcr.io/ricoberger/script_exporter:dev .
docker run --rm -it --name script_exporter -p 9469:9469 -v $(pwd)/examples:/examples ghcr.io/ricoberger/script_exporter:dev -config.file /examples/config.yaml
```

Then visit [http://localhost:9469](http://localhost:9469) in the browser of your choice. There you have access to the following examples:

- [test](http://localhost:9469/probe?script=test&prefix=test): Invalid values which are returned by the script are omitted.
- [ping](http://localhost:9469/probe?script=ping&prefix=test&params=target&target=example.com): Pings the specified address in the `target` parameter and returns if it was successful or not.
- [helloworld](http://localhost:9469/probe?script=helloworld): Returns the specified argument in `args` as label.
- [showtimeout](http://localhost:9469/probe?script=showtimeout&timeout=37): Reports whether or not the script is being run with a timeout from Prometheus, and what it is.
- [docker](http://localhost:9469/probe?script=docker): Example using `docker exec` to return the number of files in a Docker container.
- [args](http://localhost:9469/probe?script=args&params=arg3,arg4&arg3=test3&arg4=test4): Pass arguments to the script via the configuration file.
- [metrics](http://localhost:9469/metrics): Shows internal metrics from the script exporter.

You can also deploy the script_exporter to Kubernetes via Helm:

```sh
helm repo add ricoberger https://ricoberger.github.io/helm-charts
helm install script-exporter ricoberger/script-exporter
```

## Usage and configuration

The script_exporter is configured via a configuration file and command-line flags.

```txt
Usage of ./bin/script_exporter:
  -config.check
        Do not run the exporter. Only check the configuration file and exit (0 if the Configuration file is valid, 1 otherwise).
  -config.file file
        Configuration file in YAML format. (default "config.yaml")
  -create-token
        Create bearer token for authentication.
  -log.env
        Log environment variables used by a script.
  -log.format string
        Output format of log messages. One of: [logfmt, json] (default "logfmt")
  -log.level string
        Only log messages with the given severity or above. One of: [debug, info, warn, error] (default "info")
  -noargs
        Restrict script to accept arguments, for security issues
  -timeout-offset seconds
        Offset to subtract from Prometheus-supplied timeout in seconds. (default 0.5)
  -version
        Print version information.
  -web.listen-address string
        Address to listen on for web interface and telemetry. (default ":9469")
```

The configuration file is written in YAML format, defined by the scheme described below.

```yaml
tls:
  enabled: <boolean>
  crt: <string>
  key: <string>

basicAuth:
  enabled: <boolean>
  username: <string>
  password: <string>

bearerAuth:
  enabled: <boolean>
  signingKey: <string>

discovery:
  host: <string>
  port: <string>
  scheme: <string>

scripts:
  - name: <string>
    command: <string>
    args:
      - <string>
    # by default the env cannot be overwritten by query parameters.
    # If you want to change this, set this option to true
    allowEnvOverwrite: <bool>
    # optional
    env:
      <key>: <value>
    # by default the output will also be parsed when the script fails,
    # this can be changed by setting this option to true
    ignoreOutputOnFail: <boolean>
    timeout:
      # in seconds, 0 or negative means none
      max_timeout: <float>
      enforced: <boolean>
    cacheDuration: <duration>
    useExpiredCacheOnError: <boolean>
    discovery:
      params:
        <string>: <string>
      prefix: <string>
      scrape_interval: <duration>
      scrape_timeout: <duration>

scripts_configs:
  - <string>
```

The `name` of the script must be a valid Prometheus label value. The `command` string is the script which is executed with all arguments specified in `args`. To add dynamic arguments you can pass the `params` query parameter with a list of query parameters which values should be added as argument. The program will be executed directly, without a shell being invoked, and it is recommended that it be specified by path instead of relying on ``$PATH``.

The optional `env` key allows to run the script with custom environment variables.

Example: set proxy env vars for test_env script

```yaml
scripts:
  - name: test_env
    command: /tmp/my_script.sh
    env:
      http_proxy: http://proxy.example.com:3128
      https_proxy: http://proxy.example.com:3128
```

**Note:** because the program is executed directly, shell constructions can't be used.
For example:

```shell
# Error: output stream redirection (>) is a shell construction
/bin/foo >/dev/null
# Success: use appropriate command line arguments if supported by the command
/bin/foo --output /dev/null

# Error: logical operator (||) is a shell construction
/bin/foo || true
# Success: use shell interpreter with arguments
/bin/bash -c '/bin/foo || true'
# Success: or create an executable script file
/usr/local/bin/bar.sh
# Success: or run it via interpreter
/bin/bash /usr/local/bin/bar.sh
```

Prometheus will normally provide an indication of its scrape timeout to the script exporter (through a special HTTP header). This information is made available to scripts through the environment variables `$SCRIPT_TIMEOUT` and `$SCRIPT_DEADLINE`. The first is the timeout in seconds (including a fractional part) and the second is the Unix timestamp when the deadline will expire (also including a fractional part). A simple script could implement this timeout by starting with `timeout "$SCRIPT_TIMEOUT" cmd ...`. A more sophisticated program might want to use the deadline time to compute internal timeouts for various operation. If `enforced` is true, `script_exporter` attempts to enforce the timeout by killing the script's main process after the timeout expires. The default is to not enforce timeouts. If `max_timeout` is set for a script, it limits the maximum timeout value that requests can specify; a request that specifies a larger timeout will have the timeout adjusted down to the `max_timeout` value.

For testing purposes, the timeout can be specified directly as a URL parameter (`timeout`). If present, the URL parameter takes priority over the Prometheus HTTP header.

The `cacheDuration` config can be used to cache the results from an execution of the script for the provided time. The provided duration must be parsable by the [`time.ParseDuration`](https://pkg.go.dev/time#ParseDuration) function. If no cache duration is provided or the provided cache duration can not be parsed, the output of an script will not be cached. It produces the metric `script_use_cache` to track in time when results returned are coming from cache.

The `useExpiredCacheOnError` config allow to return expired cache in case of errors. It produces the metric `script_use_expired_cache` for track in time if you are using expired cache, it mean there is something wrong with the script execution.

You can fine tune the script discovery options via optional script `discovery`. All these options will go through prometheus configuration where you can change them via relabel mechanism.
There are `params` to define dynamic script parameters (with reserved keys: `params`, `prefix`, `script` and `timeout`) where only value will be used during script invoking (similar to `args`), `prefix` to define prefix for all script metrics, `scrape_interval` to define how often the script scrape should run and `scrape_timeout` to define the scrape timeout for prometheus (similar to `timeout`).

The global `discovery` configures the main discovery parameters. If not defined, the exporter will use `Host:` header from the request to decide how to present a `target` to prometheus.

## Prometheus configuration

The script_exporter needs to be passed the script name as a parameter (`script`). You can also pass a custom prefix (`prefix`) which is prepended to metrics names and the names of additional parameters which should be passed to the script (`params` and then additional URL parameters). If the `output` parameter is set to `ignore` then the script_exporter only return `script_success{}`, `script_duration_seconds{}` and `script_exit_code{}`.

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

Optionally, HTTP service discovery can be configured like this:

```yaml
- job_name: "exported-scripts"
  http_sd_configs:
  - url: http://prometheus-script-exporter:9469/discovery
```

This will make prometheus reach to `/discovery` endpoint and collect the targets. Targets are all the scripts configured in the exporter.

## Breaking changes

Changes from version 1.3.0:

- The command line flag ``-web.telemetry-path`` has been removed and its value is now always ``/probe``, which is a change from the previous default of ``/metrics``. The path ``/metrics`` now responds with Prometheus metrics for script_exporter itself.
- The command line flag ``-config.shell`` has been removed. Programs are now always run directly.
