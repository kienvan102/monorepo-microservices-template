package mongoclient

import (
	"context"
	"testing"
	"time"
)

func TestConnectOptionsDefaults(t *testing.T) {
	o := newConnectOptions("mongodb://localhost:27017", nil)
	if o.pingTimeout != defaultPingTimeout {
		t.Fatalf("pingTimeout = %v, want %v", o.pingTimeout, defaultPingTimeout)
	}
	if o.client.MaxPoolSize != nil || o.client.MinPoolSize != nil || o.client.MaxConnIdleTime != nil ||
		o.client.ServerSelectionTimeout != nil || o.client.AppName != nil {
		t.Fatalf("driver options set without options: %+v", o.client)
	}
	if o.newID != nil {
		t.Fatal("an id generator is set without WithIDGenerator")
	}
}

func TestWithIDGenerator(t *testing.T) {
	o := newConnectOptions("mongodb://localhost:27017", []Option{
		WithIDGenerator(func(context.Context) (any, error) { return "sys-1", nil }),
	})
	if o.newID == nil {
		t.Fatal("WithIDGenerator did not store the generator")
	}
	if id, err := o.newID(context.Background()); err != nil || id != "sys-1" {
		t.Fatalf("stored generator returned (%v, %v)", id, err)
	}
}

func TestConnectOptionsOverride(t *testing.T) {
	o := newConnectOptions("mongodb://localhost:27017", []Option{
		WithPingTimeout(3 * time.Second),
		WithServerSelectionTimeout(4 * time.Second),
		WithMaxPoolSize(50),
		WithMinPoolSize(5),
		WithMaxConnIdleTime(time.Minute),
		WithAppName("orders"),
	})
	if o.pingTimeout != 3*time.Second {
		t.Errorf("pingTimeout = %v", o.pingTimeout)
	}
	if o.client.ServerSelectionTimeout == nil || *o.client.ServerSelectionTimeout != 4*time.Second {
		t.Errorf("ServerSelectionTimeout = %v", o.client.ServerSelectionTimeout)
	}
	if o.client.MaxPoolSize == nil || *o.client.MaxPoolSize != 50 {
		t.Errorf("MaxPoolSize = %v", o.client.MaxPoolSize)
	}
	if o.client.MinPoolSize == nil || *o.client.MinPoolSize != 5 {
		t.Errorf("MinPoolSize = %v", o.client.MinPoolSize)
	}
	if o.client.MaxConnIdleTime == nil || *o.client.MaxConnIdleTime != time.Minute {
		t.Errorf("MaxConnIdleTime = %v", o.client.MaxConnIdleTime)
	}
	if o.client.AppName == nil || *o.client.AppName != "orders" {
		t.Errorf("AppName = %v", o.client.AppName)
	}
}

func TestOptionsOverrideURI(t *testing.T) {
	o := newConnectOptions("mongodb://localhost:27017/?maxPoolSize=10", []Option{WithMaxPoolSize(20)})
	if o.client.MaxPoolSize == nil || *o.client.MaxPoolSize != 20 {
		t.Fatalf("MaxPoolSize = %v, want 20", o.client.MaxPoolSize)
	}
}
