// Package mongoclient opens connections to locally managed mongod instances.
package mongoclient

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DefaultTimeout bounds connection and server selection for local servers, where
// anything slower than this means the server is not actually up.
const DefaultTimeout = 3 * time.Second

// URI builds a direct-connection URI for a mongod on loopback.
func URI(port int) string {
	return fmt.Sprintf("mongodb://127.0.0.1:%d/?directConnection=true", port)
}

// Connect opens a direct connection to a mongod on loopback.
//
// directConnection is essential: without it the driver performs replica set
// discovery and refuses to talk to a member that is not primary, which would
// break both health checks and the shutdown command on secondaries.
func Connect(port int, timeout time.Duration) (*mongo.Client, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	opts := options.Client().
		ApplyURI(URI(port)).
		SetServerSelectionTimeout(timeout).
		SetConnectTimeout(timeout)

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("connect to 127.0.0.1:%d: %w", port, err)
	}
	return client, nil
}

// Ping reports whether a mongod on the port answers a hello command.
func Ping(ctx context.Context, port int, timeout time.Duration) error {
	client, err := Connect(port, timeout)
	if err != nil {
		return err
	}
	defer client.Disconnect(context.Background())

	return client.Database("admin").
		RunCommand(ctx, bson.D{{Key: "hello", Value: 1}}).
		Err()
}
