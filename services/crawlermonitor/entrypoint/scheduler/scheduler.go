package scheduler

import (
	"context"
	"flag"
	"time"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/logger"
	"github.com/kienvan102/monorepo-microservices-template/core/app"
	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/business"
	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/settings"
)

type Builder func(ctx context.Context, cfg settings.Config, log logger.Logger) (*business.Monitor, func(), error)

type Handler struct {
	build Builder
}

func NewHandler(build Builder) *Handler { return &Handler{build: build} }

var _ app.Component[settings.Config] = (*Handler)(nil)

func (h *Handler) InitFlags(_ *flag.FlagSet) {}

func (h *Handler) Run(ctx context.Context, rt app.Runtime, cfg settings.Config) error {
	monitor, cleanup, err := h.build(ctx, cfg, rt.Log)
	if err != nil {
		return err
	}
	defer cleanup()

	for {
		if err := monitor.Collect(ctx); err != nil && ctx.Err() == nil {
			rt.Log.Error("metric collection completed with errors", "error", err)
		}
		if ctx.Err() != nil {
			return nil
		}

		timer := time.NewTimer(cfg.Interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		case <-timer.C:
		}
	}
}
