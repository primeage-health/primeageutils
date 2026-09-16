// Package mongodb holds the Mongo connection every service opens the same way.
//
// The deployment is one replica set per environment, reached with the same four
// environment variables everywhere, so the connect, the ping and the topology
// check are the platform's rather than any one service's.
package mongodb

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Connect dials Mongo from DB_ADDR, DB_PORT, DB_USER and DB_PASSWORD, and
// verifies the connection before returning it.
func Connect() (*mongo.Client, error) {
	uri := os.Getenv("DB_ADDR") + os.Getenv("DB_PORT")
	if uri == "" {
		return nil, fmt.Errorf("DB_ADDR and DB_PORT not set")
	}

	clientOptions := options.Client().ApplyURI(uri)
	if user := os.Getenv("DB_USER"); user != "" {
		clientOptions.SetAuth(options.Credential{
			Username: user,
			Password: os.Getenv("DB_PASSWORD"),
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("connecting to mongo at %s: %w", uri, err)
	}

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("pinging mongo at %s: %w", uri, err)
	}

	return client, nil
}

// WithTransaction runs fn inside one MongoDB transaction.
//
// This is what makes the outbox trustworthy: a state change and the event
// describing it are written together or not at all, so the audit trail can never
// silently disagree with operational state. Every mutating adapter method that
// carries an event goes through here.
func WithTransaction(ctx context.Context, client *mongo.Client, fn func(mongo.SessionContext) error) error {
	session, err := client.StartSession()
	if err != nil {
		return fmt.Errorf("starting mongo session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (any, error) {
		return nil, fn(sessCtx)
	})
	return err
}

// EnsureTransactionsAvailable verifies the deployment can run transactions.
//
// A standalone mongod cannot: multi-document transactions need a replica set,
// even a single-node one. Without this check the failure surfaces as a confusing
// error on the first write instead of at startup, so it is worth one read-only
// command at boot.
func EnsureTransactionsAvailable(ctx context.Context, client *mongo.Client) error {
	var result struct {
		SetName string `bson:"setName"`
		Msg     string `bson:"msg"`
	}

	if err := client.Database("admin").
		RunCommand(ctx, bson.D{{Key: "hello", Value: 1}}).
		Decode(&result); err != nil {
		return fmt.Errorf("checking mongo topology: %w", err)
	}

	// msg is "isdbgrid" on a mongos, which supports transactions too.
	if result.SetName == "" && result.Msg != "isdbgrid" {
		return fmt.Errorf("mongo is running standalone; the transactional outbox requires a replica set — " +
			"start mongod with --replSet rs0 and run rs.initiate() once")
	}
	return nil
}
