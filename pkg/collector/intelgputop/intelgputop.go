package intelgputop

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	kitlog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type GPUTop struct {
	logger kitlog.Logger

	powerConsumption prometheus.Counter
	engineUsage      *prometheus.CounterVec
}

type GPUTopData struct {
	Period     Period             `json:"period"`
	Frequency  Frequency          `json:"frequency"`
	Interrupts Interrupts         `json:"interrupts"`
	Rc6        Rc6                `json:"rc6"`
	Power      Power              `json:"power"`
	Engines    map[string]*Engine `json:"engines"`
}
type Period struct {
	Duration float64 `json:"duration"`
	Unit     string  `json:"unit"`
}
type Frequency struct {
	Requested float64 `json:"requested"`
	Actual    float64 `json:"actual"`
	Unit      string  `json:"unit"`
}
type Interrupts struct {
	Count float64 `json:"count"`
	Unit  string  `json:"unit"`
}
type Rc6 struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
type Power struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
type Engine struct {
	Busy float64 `json:"busy"`
	Sema float64 `json:"sema"`
	Wait float64 `json:"wait"`
	Unit string  `json:"unit"`
}

func New(r prometheus.Registerer) (*GPUTop, error) {
	return &GPUTop{
		powerConsumption: promauto.With(r).NewCounter(
			prometheus.CounterOpts{
				Name: "gpu_power_consumption_total",
				Help: "Aggregated power usage in Wh.",
			},
		),
		engineUsage: promauto.With(r).NewCounterVec(
			prometheus.CounterOpts{
				Name: "gpu_engine_seconds_total",
				Help: "Usage of GPU engines in seconds.",
			},
			[]string{"engine", "mode"},
		),
	}, nil
}

func (g *GPUTop) WithLogger(l kitlog.Logger) *GPUTop {
	g.logger = kitlog.With(l, "collector", g.Name())
	return g
}

func (g *GPUTop) Name() string {
	return "gputop"
}

func (g *GPUTop) record(d *GPUTopData) error {
	duration, err := time.ParseDuration(fmt.Sprintf(
		"%f%s",
		d.Period.Duration,
		d.Period.Unit,
	))
	if err != nil {
		return err
	}

	if d.Power.Unit == "W" {
		g.powerConsumption.Add(duration.Hours() * d.Power.Value)
	} else {
		return fmt.Errorf("unexpected unit for power consumption: %s", d.Power.Unit)
	}

	for engine, values := range d.Engines {
		if values.Unit != "%" {
			return fmt.Errorf("unexpected unit for engine %s: %s", engine, values.Unit)
		}
		g.engineUsage.WithLabelValues(
			strings.ToLower(engine), "busy",
		).Add(duration.Seconds() * values.Busy * 0.01)
		g.engineUsage.WithLabelValues(
			strings.ToLower(engine), "sema",
		).Add(duration.Seconds() * values.Sema * 0.01)
		g.engineUsage.WithLabelValues(
			strings.ToLower(engine), "wait",
		).Add(duration.Seconds() * values.Wait * 0.01)
	}

	return nil
}

func (g *GPUTop) Run() error {
	cmd := exec.Command("intel_gpu_top", "-J", "-o", "-")

	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	bufOut := bufio.NewReader(out)
	dec := json.NewDecoder(bufOut)

	var data GPUTopData

	for {
		if err := dec.Decode(&data); err != nil {
			return err
		}

		if err := g.record(&data); err != nil {
			output, _ := json.Marshal(&data)
			_ = level.Error(g.logger).Log("msg", "unable to process intel_gpu_top output", "output", output, "err", err)
		}

		// the json stream separates the JSON objects with commas, this will read a comma if there is one
		b, err := bufOut.ReadByte()
		if err != nil {
			return err
		}
		if b != ',' {
			bufOut.UnreadByte()
		} else {
			dec = json.NewDecoder(bufOut)
		}

	}

}
