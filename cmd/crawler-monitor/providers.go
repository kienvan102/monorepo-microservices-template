package main

import (
	"context"

	"github.com/kienvan102/monorepo-microservices-template/commonlib/logger"
	"github.com/kienvan102/monorepo-microservices-template/commonlib/mongoclient"
	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/settings"
)

func provideMongoClient(ctx context.Context, cfg settings.Config, logger logger.Logger) (mongoclient.Client, func(), error) {
	client, err := mongoclient.NewConnector().Connect(
		ctx,
		cfg.Mongo.URI,
		cfg.Mongo.Database,
		mongoclient.WithPingTimeout(cfg.Mongo.PingTimeout),
	)
	if err != nil {
		logger.Error("Failed to connect to MongoDB", "error", err)
		return nil, nil, err
	}
	logger.Info("Connected to MongoDB", "uri", cfg.Mongo.URI, "database", cfg.Mongo.Database)
	return client, client.Disconnect, nil
}