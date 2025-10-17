# Script Exporter

The Script Exporter is a [Prometheus](https://prometheus.io) exporter to execute
scripts and collect metrics from the output or the exit status. The scripts to
be executed are defined via a configuration file. In the configuration file
several scripts can be specified. The script which should be executed is
indicated by a parameter in the scrap configuration. The output of the script is
captured and is provided for Prometheus. Even if the script does not produce any
output, the exit status and the duration of the execution are provided.

## Building and Running

To run the Script Exporter you can use the one of the binaries from the
[release](https://github.com/ricoberger/script_exporter/releases) page or the
[Docker image](https://github.com/ricoberger/script_exporter/pkgs/container/script_exporter).
You can also build the Script Exporter by yourself by running the following
commands:

```sh
git clone https://github.com/ricoberger/script_exporter.git
cd script_exporter
make build
```

Afterwards you can run the Script Exporter with the
[example configuration](./scripts.yaml), by using the following command:

```sh
./bin/script_exporter
```

To run the examples via Docker or Docker Compose the following commands can be
used. The Docker Compose setup will also start a Prometheus instance with a
[scrape configuration](./prometheus.yaml) for all scripts.

```sh
# Docker
docker build -f ./Dockerfile -t ghcr.io/ricoberger/script_exporter:latest .
docker run --rm -it --name script_exporter -p 9469:9469 -v $(pwd)/scripts.yaml:/script_exporter/scripts.yaml -v $(pwd)/prober/scripts:/script_exporter/prober/scripts ghcr.io/ricoberger/script_exporter:latest

# Docker Compose
docker compose -f docker-compose.yaml up --build --force-recreate
```

Then visit [http://localhost:9469](http://localhost:9469) in the browser of your
choice. There you have access to the following examples:

- [output](http://localhost:9469/probe?script=output): Parses the returned
  output from the script and only return valid Prometheus metrics.
- [ping](http://localhost:9469/probe?script=ping&prefix=test&params=target&target=example.com):
  Pings the specified address in the `target` parameter and returns if it was
  successful or not.
- [showtimeout](http://localhost:9469/probe?script=showtimeout&timeout=42):
  Reports whether or not the script is being run with a timeout from Prometheus,
  and what it is.
- [docker](http://localhost:9469/probe?script=docker): Example using
  `docker exec` to return the number of files in a Docker container.
- [sleep](http://localhost:9469/probe?script=sleep&params=seconds&seconds=20):
  Execute a script, which executes a `sleep` command with the duration provided
  in the `seconds` parameter. The command will be canceled after 10 seconds.
- [cache](http://localhost:9469/probe?script=cache&params=seconds&seconds=5):
  Execute a script, which executes a `sleep` command with the duration provided
  in the `seconds` parameter. The output of the script will be cached for 60
  seconds, so that follow up requests will be faaster.

You can also deploy the Script Exporter to Kubernetes via Helm:

```sh
helm upgrade --install script-exporter oci://ghcr.io/ricoberger/charts/script-exporter --version <VERSION>
```

## Usage and Configuration

The Script Exporter is configured via a configuration file and command-line
flags (such as what configuration file to load, what port to listen on, and the
logging format and level).

The Script Exporter can reload its configuration file at runtime. If the new
configuration is not well-formed, the changes will not be applied. A
configuration reload is triggered by sending a `SIGHUP` to the Script Exporter
process or by sending a HTTP POST request to the `/-/reload` endpoint.

### Command-Line Flags

```plaintext
usage: script_exporter [<flags>]


Flags:
  -h, --[no-]help                Show context-sensitive help (also try --help-long and --help-man).
      --config.files="scripts.yaml"
                                 Configuration files. To specify multiple configuration files glob patterns can be used.
      --config.reload-interval=1h
                                 Reload interval of the configuration file.
      --[no-]config.check        If true, validate the configuration files and then exit.
      --[no-]log.env             If true, environment variables passed to a script will be logged.
      --[no-]script.no-args      Restrict script to accept arguments.
      --script.timeout-offset=0.5
                                 Offset to subtract from timeout in seconds.
      --web.external-url=<url>   The URL under which Script Exporter is externally reachable (for example, if Script Exporter is served via a reverse proxy). Used for generating relative and absolute links back to Script Exporter itself. If the URL has a path portion, it will be used to prefix all HTTP endpoints served by Script Exporter. If omitted, relevant URL components
                                 will be derived automatically.
      --web.route-prefix=<path>  Prefix for the internal routes of web endpoints. Defaults to path of --web.external-url.
      --discovery.host=""        Host for service discovery.
      --discovery.port=""        Port for service discovery.
      --discovery.scheme=""      Scheme for service discovery.
      --web.listen-address=:9469 ...
                                 Addresses on which to expose metrics and web interface. Repeatable for multiple addresses. Examples: `:9100` or `[::1]:9100` for http, `vsock://:9100` for vsock
      --web.config.file=""       Path to configuration file that can enable TLS or authentication. See: https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md
      --log.level=info           Only log messages with the given severity or above. One of: [debug, info, warn, error]
      --log.format=logfmt        Output format of log messages. One of: [logfmt, json]
      --[no-]version             Show application version.
```

### Configuration File

The scripts for the Script Exporter can be configured via multiple configuration
files. The configuration files can be set via the `--config.files` command-line
flag. To split the scripts accross multiple configuration files a glob pattern
can be used, e.g. `--config.files=./scripts/*.yaml`.

```yaml
scripts:
  - # The name of the script. To run the selected script within a probe the
    # "script" parameter must be set in the Prometheus scrape configuration.
    name: <string>
    # The command which should be run. This could be the path to a shell script
    # or any other valid command which is available within your system.
    command:
      - <string>
    # Additional arguments which should be passed to the command. The arguments
    # are passed to the command first, afterwards all additional arguments
    # specified by the "params" parameter from the Prometheus scrape config are
    # passed to the command: "<COMMAND> [<ARGUMENTS>] [<PARAMS>]".
    args:
      - <string>
    # All additional environment variables which should be passed to the script,
    # besides the globally defined environment variables on the system, where
    # Script Exporter is running.
    #
    # The parameters defined via the "params" query parameter are also passed to
    # script as environemnt variables. If an environemnt variable with the same
    # name as a parameter is already defined it will not be overwritten, unless
    # the "allow_env_overwrites" options is set to "true".
    env:
      <string>: <string>
    allow_env_overwrite: <boolean>
    # If set to "true" the command will be executed with privileged (root)
    # permissions by executing the "command" with a pre-fixed "sudo":
    # "sudo <COMMAND> [<ARGUMENTS>] [<PARAMS>]"
    #
    # Note that you still need to create the relevant sudoers entries, Script
    # Exporter will not do this for you.
    sudo: <boolean>
    # By default the output of a script will be checked for valid Prometheus
    # metrics. These metrics will be exported in addition to the default script
    # metrics.
    output:
      # If set to "true" the output of a script will be ignored and only the
      # default metrics will be exported.
      ignore: <boolean>
      # If set to "true" the output of a script will be ignored if the script
      # returned an error and only the default metrics will be exported.
      ignore_on_error: <boolean>
    # Timeout configuration for the script. By default the timeout specified via
    # the "timeout" parameter or the "scrape_timeout" Prometheus configuration
    # will be used.
    #
    # We add the offset defined via the "--script.timeout-offset" command-line
    # flag to these timeouts.
    #
    # This information is made available to scripts through the environment
    # variables "$SCRIPT_TIMEOUT" and "$SCRIPT_DEADLINE". The first is the
    # timeout in seconds (including a fractional part) and the second is the
    # Unix timestamp when the deadline will expire (also including a fractional
    # part).
    timeout:
      # Set a max timeout in seconds. If the Prometheus specific timeout is
      # larger then the max timeout, the max timeout will be used.
      max_timeout: <float>
      # If set to "true" the timeout will be enforced, otherwise the script will
      # continue with running, also when the timeout is passed.
      enforced: <boolean>
      # Set a wait delay in seconds.
      #
      # If the script spawns a child process (e.g. "sleep") the timeout might
      # not be enforced, because Go waits for the timeout and the closing of all
      # I/O pipes. To enforce the timeout for such cases the "wait_delay" must
      # be set to a low value (e.g. "0.01")
      wait_delay: <float>
    # By default the result of a script execution will not be cached. To reuse
    # the result from one scrape in a follow up scrape the "duration" must be
    # set.
    #
    # Note: The cache is not presisted, which means that the cache is deleted,
    # if the Script Exporter is restarted.
    cache:
      # Cache duration in seconds. If this is set, the result of a script
      # execution will be returned from the cache instead of running the script
      # again.
      duration: <float>
      # If set to "true" also the result of a script execution which returned an
      # error will be cached.
      cache_on_error: <boolean>
      # If set to "true" the result from the cache will be returned, when the
      # script returned an error, also when the cache entry is already expired.
      use_expired_cache_on_error: <boolean>
    # Configuration for the Prometheus discovery.
    discovery:
      # A list of parameters which will be passed to the script and within the
      # "params" query parameter.
      params:
        <string>: <string>
      # The scrape interval and scrape timeout which should be used by
      # Prometheus for the discovered script. If not set the global scrape
      # interval and timeout will be used.
      scrape_interval: <duration>
      scrape_timeout: <duration>
```

### TLS and Basic Authentication

The Script Exporter supports TLS and basic authentication. This enables better
control of the various HTTP endpoints. To use TLS and/or basic authentication,
you need to pass a configuration file using the `--web.config.file` parameter.
The format of the file is described
[in the exporter-toolkit repository](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md).

Note that the TLS and basic authentication settings affect all HTTP endpoints:
`/metrics` for scraping, `/probe` for probing, and the web UI.

### Prometheus Configuration

An example configuration for Prometheus can be found in the
[prometheus.yaml](./prometheus.yaml) file, which contains one job for each
example script, a job to scrape the Script Exporter metrics and a job to use the
Prometheus discovery feature of the script exporter.

```yaml
scrape_configs:
  # Scrape configuration for all of the example scripts.
  - job_name: showtimeout
    metrics_path: /probe
    params:
      script:
        - showtimeout
    static_configs:
      - targets:
          - localhost:9469

  # Configuration to get the metrics of the Script Exporter.
  - job_name: "script_exporter"
    metrics_path: /metrics
    static_configs:
      - targets:
          - localhost:9469

  # Configuration for the Prometheus discovery feature of the Script Exporter.
  - job_name: scripts
    http_sd_configs:
      - url: http://localhost:9469/discovery
```

By default the Script Exporter will use the host, port and scheme used by
Prometheus when creating the targets for the Prometheus discovery. If you want
to overwrite the host, port and scheme the `--discovery.host`,
`--discovery.port` and `--discovery.scheme` command-line flags can be set.
