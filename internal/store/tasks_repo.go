package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"task-api-huma-mongo/internal/service"
)

type MongoTaskRepository struct {
	client     *mongo.Client
	collection *mongo.Collection
	timeout    time.Duration
}

type taskDocument struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Title     string             `bson:"title"`
	Done      bool               `bson:"done"`
	Tags      []string           `bson:"tags,omitempty"`
	CreatedAt time.Time          `bson:"createdAt"`
}

func NewMongoTaskRepository(store *MongoStore) *MongoTaskRepository {
	return &MongoTaskRepository{
		client:     store.client,
		collection: store.collection,
		timeout:    store.timeout,
	}
}

func (r *MongoTaskRepository) Create(ctx context.Context, task service.Task) (*service.Task, error) {
	doc := taskDocument{
		ID:        primitive.NewObjectID(),
		Title:     task.Title,
		Done:      task.Done,
		Tags:      task.Tags,
		CreatedAt: task.CreatedAt,
	}

	opCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	if _, err := r.collection.InsertOne(opCtx, doc); err != nil {
		return nil, err
	}

	task.ID = doc.ID.Hex()
	return &task, nil
}

func (r *MongoTaskRepository) Get(ctx context.Context, id string) (*service.Task, error) {
	objID, err := parseObjectID(id)
	if err != nil {
		return nil, err
	}

	opCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	var doc taskDocument
	if err := r.collection.FindOne(opCtx, bson.M{"_id": objID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, service.ErrNotFound
		}
		return nil, err
	}

	task := toTask(doc)
	return &task, nil
}

func (r *MongoTaskRepository) List(ctx context.Context, filter service.TaskFilter) ([]service.Task, error) {
	query := bson.D{}
	if filter.Done != nil {
		query = append(query, bson.E{Key: "done", Value: *filter.Done})
	}
	if filter.Tag != "" {
		query = append(query, bson.E{Key: "tags", Value: filter.Tag})
	}

	opCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	cur, err := r.collection.Find(opCtx, query)
	if err != nil {
		return nil, err
	}
	defer cur.Close(opCtx)

	var tasks []service.Task
	for cur.Next(opCtx) {
		var doc taskDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		tasks = append(tasks, toTask(doc))
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *MongoTaskRepository) Update(ctx context.Context, id string, update service.UpdateTaskRequest) (*service.Task, error) {
	objID, err := parseObjectID(id)
	if err != nil {
		return nil, err
	}

	set := bson.D{}
	if update.Title != nil {
		set = append(set, bson.E{Key: "title", Value: *update.Title})
	}
	if update.Done != nil {
		set = append(set, bson.E{Key: "done", Value: *update.Done})
	}
	if update.Tags != nil {
		set = append(set, bson.E{Key: "tags", Value: *update.Tags})
	}
	if len(set) == 0 {
		return nil, &service.ValidationError{Field: "body", Message: "at least one field must be provided"}
	}

	opCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc taskDocument
	if err := r.collection.FindOneAndUpdate(
		opCtx,
		bson.M{"_id": objID},
		bson.D{{Key: "$set", Value: set}},
		opts,
	).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, service.ErrNotFound
		}
		return nil, err
	}

	task := toTask(doc)
	return &task, nil
}

func (r *MongoTaskRepository) Delete(ctx context.Context, id string) error {
	objID, err := parseObjectID(id)
	if err != nil {
		return err
	}

	opCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	res, err := r.collection.DeleteOne(opCtx, bson.M{"_id": objID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return service.ErrNotFound
	}
	return nil
}

func (r *MongoTaskRepository) Ping(ctx context.Context) error {
	opCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.client.Ping(opCtx, readpref.Primary())
}

func parseObjectID(id string) (primitive.ObjectID, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("%w: %s", service.ErrInvalidID, id)
	}
	return objID, nil
}

func toTask(doc taskDocument) service.Task {
	return service.Task{
		ID:        doc.ID.Hex(),
		Title:     doc.Title,
		Done:      doc.Done,
		Tags:      doc.Tags,
		CreatedAt: doc.CreatedAt,
	}
}
