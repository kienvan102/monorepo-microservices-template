package mongoclient

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const defaultPingTimeout = 10 * time.Second

type Option func(*connectOptions)

type connectOptions struct {
	client      *options.ClientOptions
	pingTimeout time.Duration
	newID       IDGenerator
}

func newConnectOptions(uri string, opts []Option) connectOptions {
	o := connectOptions{
		client:      options.Client().ApplyURI(uri),
		pingTimeout: defaultPingTimeout,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func WithPingTimeout(d time.Duration) Option {
	return func(o *connectOptions) { o.pingTimeout = d }
}

func WithServerSelectionTimeout(d time.Duration) Option {
	return func(o *connectOptions) { o.client.SetServerSelectionTimeout(d) }
}

func WithMaxPoolSize(n uint64) Option {
	return func(o *connectOptions) { o.client.SetMaxPoolSize(n) }
}

func WithMinPoolSize(n uint64) Option {
	return func(o *connectOptions) { o.client.SetMinPoolSize(n) }
}

func WithMaxConnIdleTime(d time.Duration) Option {
	return func(o *connectOptions) { o.client.SetMaxConnIdleTime(d) }
}

func WithAppName(name string) Option {
	return func(o *connectOptions) { o.client.SetAppName(name) }
}

func WithIDGenerator(gen IDGenerator) Option {
	return func(o *connectOptions) { o.newID = gen }
}
