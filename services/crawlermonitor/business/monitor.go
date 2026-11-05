package business

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/logger"
	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/entity"
	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/settings"
)

type Monitor struct {
	cfg settings.Config
	log logger.Logger
	metrics MetricData
}

func NewMonitor(
	cfg settings.Config,
	log logger.Logger,
	metric MetricData,
) *Monitor {
	return &Monitor{
		cfg: cfg,
		log: log,
		metrics: metric,
	}
}

func (m *Monitor) Collect(ctx context.Context) error {
	started := time.Now()
	results := make([]entity.Result, 0, 32)

	for _, collect := range []func(context.Context) []entity.Result{
		// m.youtube.Collect,
		// m.threads.Collect,
		// m.news.Collect,
	} {
		if err := ctx.Err(); err != nil {
			return err
		}
		results = append(results, collect(ctx)...)
	}

	failures := make([]error, 0)
	for _, result := range results {
		if result.Err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", result.ID, result.Err))
		}
	}

	if err := m.metrics.Write(ctx, results); err != nil {
		failures = append(failures, fmt.Errorf("write metrics: %w", err))
	}

	m.log.Info(
		"metric collection completed",
		"metrics", len(results),
		"failures", len(failures),
		"duration_ms", time.Since(started).Milliseconds(),
	)

	return errors.Join(failures...)
}