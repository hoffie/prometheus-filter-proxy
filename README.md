# prometheus-filter-proxy
This tool provides a Prometheus-style HTTP API, rewrites all incoming PromQL queries and forwards them to an upstream Prometheus server.

## Features
prometheus-filter-proxy sits between a Prometheus server and a Prometheus API consumer such as [Grafana](https://grafana.org).
Instead of pointing Grafana to one Prometheus Data Source, you will create multiple Grafana Organizations with their own Prometheus Data Sources.
Each Data Source points to an individual prometheus-filter-proxy endpoint which rewrites all requests to be limited to the given label matcher.
Example: `http://127.0.0.1:9090/owner=".*<somebody>.*"/`

In order for this to make any sense, your Prometheus data has to be classified with a useful label (such as `owner` in the above case).
Basically, this is an implementation of the idea of [Prometheus issue #1813](https://github.com/prometheus/prometheus/issues/1813).

## Status
This project is considered feature-complete.
It is recommended that long-term tests be run before putting this tool into production.

## Build
This tool is built using Go (tested with 1.9.2 or newer).
It makes use of some popular Go libraries, which have been vendored (using `go mod vendor`) to allow for reproducible builds and simplified cloning.

`go get -u github.com/hoffie/prometheus-filter-proxy`

## Configuration
prometheus-filter-proxy is configured using command line options only (see `--help` and the example below):

```bash
$ ./prometheus-filter-proxy --proxy.listen-addr 127.0.0.1:8888 --upstream.addr 127.0.0.1:9091
```

## License
This software is released under the [Apache 2.0 license](LICENSE).

## Author
prometheus-filter-proxy has been created by [Christian Hoffmann](https://hoffmann-christian.info/).
If you find this project useful, please star it or drop me a short [mail](mailto:mail@hoffmann-christian.info).
