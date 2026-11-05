package prometheus

import (
	"context"
	"fmt"
	"sync"

	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/entity"
	prom "github.com/prometheus/client_golang/prometheus"
)

type Data struct {
	mu       sync.Mutex
	gauges   map[string]prom.Gauge
	success  *prom.GaugeVec
	duration *prom.GaugeVec
}

func New() *Data {
	success := prom.NewGaugeVec(
		prom.GaugeOpts{Name: "crawler_monitor_query_success", Help: "1 when the latest metric query succeeded, otherwise 0."},
		[]string{"query_id"},
	)
	duration := prom.NewGaugeVec(
		prom.GaugeOpts{Name: "crawler_monitor_query_duration_seconds", Help: "Duration of the latest metric query execution."},
		[]string{"query_id"},
	)
	return &Data{
		gauges:   make(map[string]prom.Gauge),
		success:  registerGaugeVec(success),
		duration: registerGaugeVec(duration),
	}
}

func (d *Data) Write(ctx context.Context, results []entity.Result) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, result := range results {
		if err := ctx.Err(); err != nil {
			return err
		}
		d.duration.WithLabelValues(result.ID).Set(float64(result.DurationMS) / 1000)
		if result.Err != nil {
			d.success.WithLabelValues(result.ID).Set(0)
			continue
		}
		d.success.WithLabelValues(result.ID).Set(1)
		for _, metric := range result.Metrics {
			gauge, ok := d.gauges[metric.Name]
			if !ok {
				registered, err := registerGauge(prom.NewGauge(prom.GaugeOpts{Name: metric.Name}))
				if err != nil {
					return fmt.Errorf("register %s: %w", metric.Name, err)
				}
				gauge = registered
				d.gauges[metric.Name] = gauge
			}
			gauge.Set(metric.Value)
		}
	}
	return nil
}

func registerGaugeVec(gauge *prom.GaugeVec) *prom.GaugeVec {
	if err := prom.Register(gauge); err != nil {
		if already, ok := err.(prom.AlreadyRegisteredError); ok {
			if existing, ok := already.ExistingCollector.(*prom.GaugeVec); ok {
				return existing
			}
		}
		panic(err)
	}
	return gauge
}

func registerGauge(gauge prom.Gauge) (prom.Gauge, error) {
	if err := prom.Register(gauge); err != nil {
		if already, ok := err.(prom.AlreadyRegisteredError); ok {
			existing, ok := already.ExistingCollector.(prom.Gauge)
			if !ok {
				return nil, fmt.Errorf("existing collector has incompatible type")
			}
			return existing, nil
		}
		return nil, err
	}
	return gauge, nil
}
