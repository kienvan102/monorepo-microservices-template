// Package mongoclient connects to MongoDB the same way for every service and
// offers three ways to reach the data: Client.Database for database commands,
// Collection for generic typed CRUD, and RawCollection or Collection.Raw for
// the full driver API. Business-specific rules, such as which collections must
// exist, belong to the service that builds on top.
package mongoclient

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Database is the subset of MongoDB operations callers need. Both methods
// take an already deadline-bound ctx; timeouts are the caller's job.
type Database interface {
	Name() string
	RunCommand(ctx context.Context, command bson.D) (bson.Raw, error)
	RunCommandCursor(ctx context.Context, command bson.D) ([]bson.Raw, error)
}

// Client is a connected, pinged MongoDB session.
type Client interface {
	Database() Database
	Ping(ctx context.Context) error
	Disconnect()
}

type Connector interface {
	Connect(ctx context.Context, uri, database string, opts ...Option) (Client, error)
}

func NewConnector() Connector { return driverConnector{} }

type driverConnector struct{}

func (driverConnector) Connect(ctx context.Context, uri, database string, opts ...Option) (Client, error) {
	o := newConnectOptions(uri, opts)
	client, err := mongo.Connect(o.client)
	if err != nil {
		return nil, fmt.Errorf("configure MongoDB: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, o.pingTimeout)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		disconnect(client)
		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}
	return driverClient{client: client, database: client.Database(database), newID: o.newID}, nil
}

func disconnect(client *mongo.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = client.Disconnect(ctx)
}

type driverClient struct {
	client   *mongo.Client
	database *mongo.Database
	newID    IDGenerator
}

func (c driverClient) Database() Database             { return driverDatabase{database: c.database} }
func (c driverClient) Ping(ctx context.Context) error { return c.client.Ping(ctx, nil) }
func (c driverClient) Disconnect()                    { disconnect(c.client) }

type driverDatabase struct {
	database *mongo.Database
}

func (d driverDatabase) Name() string { return d.database.Name() }

func (d driverDatabase) RunCommand(ctx context.Context, command bson.D) (bson.Raw, error) {
	return d.database.RunCommand(ctx, command).Raw()
}

// RunCommandCursor drains the cursor fully: callers get one Extended JSON
// document back, so there's no streaming consumer to hand a live cursor to.
func (d driverDatabase) RunCommandCursor(ctx context.Context, command bson.D) ([]bson.Raw, error) {
	cursor, err := d.database.RunCommandCursor(ctx, command)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cursor.Close(closeCtx)
	}()
	documents := []bson.Raw{}
	if err := cursor.All(ctx, &documents); err != nil {
		return nil, err
	}
	return documents, nil
}
