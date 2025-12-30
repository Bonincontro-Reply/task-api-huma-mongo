package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoStore struct {
	client     *mongo.Client
	db         *mongo.Database
	collection *mongo.Collection
	timeout    time.Duration
}

func NewMongoStore(ctx context.Context, uri, dbName, collectionName string, timeout time.Duration) (*MongoStore, error) {
	connectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	store := &MongoStore{
		client:     client,
		db:         client.Database(dbName),
		collection: client.Database(dbName).Collection(collectionName),
		timeout:    timeout,
	}

	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}

	return store, nil
}

func (s *MongoStore) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.client.Ping(pingCtx, readpref.Primary())
}

func (s *MongoStore) Disconnect(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

func (s *MongoStore) Collection() *mongo.Collection {
	return s.collection
}
