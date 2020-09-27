package sysfs

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	kitlog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const sysFSPath = "/sys/kernel/debug/dri/0/i915_frequency_info"

type gpuFreq struct {
	ActualFrequency float64
	IdleFrequency   float64
	MaxFrequency    float64
}

func (f *gpuFreq) Buckets(step float64) []float64 {
	return prometheus.LinearBuckets(
		f.IdleFrequency,
		step,
		int((f.MaxFrequency-f.IdleFrequency)/step),
	)
}

type SysFS struct {
	logger kitlog.Logger

	frequency prometheus.Histogram
}

func parseFreq(s string) (float64, error) {
	parts := strings.Split(strings.TrimSpace(s), " ")
	if len(parts) != 2 {
		return -1, fmt.Errorf("unexpected input '%s'", s)
	}
	v, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return -1, err
	}
	return float64(v) * 1e6, nil
}

func readFrequencySysFS() (*gpuFreq, error) {
	file, err := os.Open(sysFSPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var result gpuFreq

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := parts[1]
		switch key {
		case "Actual freq":
			f, err := parseFreq(value)
			if err != nil {
				return nil, err
			}
			result.ActualFrequency = f
		case "Max freq":
			f, err := parseFreq(value)
			if err != nil {
				return nil, err
			}
			result.MaxFrequency = f
		case "Idle freq":
			f, err := parseFreq(value)
			if err != nil {
				return nil, err
			}
			result.IdleFrequency = f
		default:
			break
		}
	}

	return &result, nil
}

func New(r prometheus.Registerer) (*SysFS, error) {
	frequencies, err := readFrequencySysFS()
	if err != nil {
		return nil, err
	}

	return &SysFS{
		logger: kitlog.NewNopLogger(),
		frequency: promauto.With(r).NewHistogram(prometheus.HistogramOpts{
			Name:    "gpu_frequency",
			Help:    "GPU Frequecy in Hz",
			Buckets: frequencies.Buckets(50e6),
		}),
	}, nil
}

func (s *SysFS) WithLogger(l kitlog.Logger) *SysFS {
	s.logger = kitlog.With(l, "collector", s.Name())
	return s
}

func (s *SysFS) Name() string {
	return "sysfs"
}

func (s *SysFS) Run() error {
	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		for {
			select {
			case <-ticker.C:
				f, err := readFrequencySysFS()
				if err != nil {
					_ = level.Error(s.logger).Log("msg", "error reading frequency", "err", err)
					continue
				}
				s.frequency.Observe(f.ActualFrequency)
			}
		}
	}()

	return nil
}
