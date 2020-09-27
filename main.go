package main

import (
	"flag"
	"net/http"
	"os"
	"sync"

	kitlog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/simonswine/intel-gpu-exporter/pkg/collector/intelgputop"
	"github.com/simonswine/intel-gpu-exporter/pkg/collector/sysfs"
)

var listenAddress = flag.String("listen-address", ":8282", "The address to listen on for HTTP requests.")
var metricsPath = flag.String("metrics-path", "/metrics", "Path under which to expose metrics.")

func main() {
	logger := kitlog.NewLogfmtLogger(kitlog.NewSyncWriter(os.Stderr))
	logger = kitlog.With(logger, "ts", kitlog.DefaultTimestampUTC, "caller", kitlog.DefaultCaller)
	flag.Parse()

	logFatal := func(msg string, err error) {
		level.Error(logger).Log("msg", msg, "error", err)
		os.Exit(1)
	}

	type collector interface {
		Run() error
		Name() string
	}

	var collectors []collector
	reg := prometheus.DefaultRegisterer

	if c, err := intelgputop.New(reg); err != nil {
		_ = level.Error(logger).Log("msg", "error starting collector intelgputop", "error", err)
	} else {
		collectors = append(collectors, c.WithLogger(logger))
	}
	if c, err := sysfs.New(reg); err != nil {
		_ = level.Error(logger).Log("msg", "error starting collector intelgputop", "error", err)
	} else {
		collectors = append(collectors, c.WithLogger(logger))
	}

	var wg sync.WaitGroup

	for _, c := range collectors {
		wg.Add(1)
		go func(c collector) {
			defer wg.Done()
			if err := c.Run(); err != nil {
				_ = level.Error(logger).Log("msg", "error running collector "+c.Name(), "error", err)
			}
		}(c)
	}

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Intel GPU Exporter</title></head>
			<body>
			<h1>Intel GPU Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})

	level.Info(logger).Log("msg", "listening on metrics pulls on "+*listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		logFatal("failed to start http server", err)
	}
}
