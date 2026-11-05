package main

import (
	"context"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/logger"
	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/business"
	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/datalayer/prometheus"
	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/settings"
)

func Initialize(ctx context.Context, cfg settings.Config, logger logger.Logger) (*business.Monitor, func(), error) {
	_, cleanupMongo, err := provideMongoClient(ctx, cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		cleanupMongo()
	}
	newMetricData := prometheus.New()
	monitor := business.NewMonitor(cfg, logger, newMetricData)
	return monitor, cleanup, nil	
}