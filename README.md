# Intel GPU exporter

Export Intel's GPU stats as Prometheus metrics


Collectors:

- `intel_gpu_top` for engine usage and power consumption
- `` for frequency stats

## Usage

```
  -listen-address string
        The address to listen on for HTTP requests. (default ":8282")
  -metrics-path string
        Path under which to expose metrics. (default "/metrics")
```
