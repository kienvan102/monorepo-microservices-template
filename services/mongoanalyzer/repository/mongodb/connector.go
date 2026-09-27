// Package mongodb adds this analyzer's one business rule on top of the
// generic core/mongoclient: the configured collection must exist.
package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/kienvan102/monorepo-microservices-template/core/mongoclient"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/settings"
)

// Connector opens a MongoDB connection for cfg and confirms cfg.Collection
// exists before returning, so callers never hold a Client pointed at a
// missing collection.
type Connector struct {
	inner mongoclient.Connector
}

func NewConnector() *Connector { return &Connector{inner: mongoclient.NewConnector()} }

func (c *Connector) Connect(ctx context.Context, cfg settings.MongoConfig) (mongoclient.Client, error) {
	client, err := c.inner.Connect(ctx, cfg.URI, cfg.Database, mongoclient.WithPingTimeout(cfg.QueryTimeout))
	if err != nil {
		return nil, err
	}

	queryCtx, cancel := context.WithTimeout(ctx, cfg.QueryTimeout)
	defer cancel()
	raw, err := client.Database().RunCommand(queryCtx, bson.D{
		{Key: "listCollections", Value: 1},
		{Key: "filter", Value: bson.D{{Key: "name", Value: cfg.Collection}}},
	})
	if err != nil {
		client.Disconnect()
		return nil, fmt.Errorf("check collection: %w", err)
	}
	var reply struct {
		Cursor struct {
			FirstBatch []bson.Raw `bson:"firstBatch"`
		} `bson:"cursor"`
	}
	if err := bson.Unmarshal(raw, &reply); err != nil {
		client.Disconnect()
		return nil, fmt.Errorf("check collection: %w", err)
	}
	if len(reply.Cursor.FirstBatch) == 0 {
		client.Disconnect()
		return nil, fmt.Errorf("collection %s does not exist in database %s", cfg.Collection, cfg.Database)
	}
	return client, nil
}
