package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"task-api-huma-mongo/internal/config"
	"task-api-huma-mongo/internal/seed"
	"task-api-huma-mongo/internal/store"
)

const (
	defaultMongoURI        = "mongodb://localhost:27017"
	defaultMongoDB         = "taskdb"
	defaultMongoCollection = "tasks"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := loadConfig()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	mongoStore, err := store.NewMongoStore(ctx, cfg.MongoURI, cfg.MongoDB, cfg.MongoCollection, cfg.Timeout)
	if err != nil {
		slog.Error("mongo connect error", "err", err)
		os.Exit(1)
	}
	defer func() {
		_ = mongoStore.Disconnect(context.Background())
	}()

	result, err := seed.Run(ctx, mongoStore.Collection(), cfg.Seed)
	if err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}

	slog.Info("seed completed",
		"inserted", result.Inserted,
		"updated", result.Updated,
		"deleted", result.Deleted,
	)
}

type AppConfig struct {
	MongoURI        string
	MongoDB         string
	MongoCollection string
	Seed            seed.Config
	Timeout         time.Duration
}

func loadConfig() (AppConfig, error) {
	timeout, err := config.DurationEnv("SEED_TIMEOUT", 30*time.Second)
	if err != nil {
		return AppConfig{}, fmt.Errorf("invalid SEED_TIMEOUT: %w", err)
	}

	count, err := config.IntEnv("SEED_COUNT", 50)
	if err != nil {
		return AppConfig{}, fmt.Errorf("invalid SEED_COUNT: %w", err)
	}
	randomSeed, err := config.Int64Env("SEED_RANDOM_SEED", 1)
	if err != nil {
		return AppConfig{}, fmt.Errorf("invalid SEED_RANDOM_SEED: %w", err)
	}
	doneRatio, err := config.FloatEnv("SEED_DONE_RATIO", 0.3)
	if err != nil {
		return AppConfig{}, fmt.Errorf("invalid SEED_DONE_RATIO: %w", err)
	}
	tagMin, err := config.IntEnv("SEED_TAG_COUNT_MIN", 0)
	if err != nil {
		return AppConfig{}, fmt.Errorf("invalid SEED_TAG_COUNT_MIN: %w", err)
	}
	tagMax, err := config.IntEnv("SEED_TAG_COUNT_MAX", 2)
	if err != nil {
		return AppConfig{}, fmt.Errorf("invalid SEED_TAG_COUNT_MAX: %w", err)
	}
	createdAtStart, err := parseOptionalTime("SEED_CREATED_AT_START")
	if err != nil {
		return AppConfig{}, err
	}
	createdAtEnd, err := parseOptionalTime("SEED_CREATED_AT_END")
	if err != nil {
		return AppConfig{}, err
	}

	return AppConfig{
		MongoURI:        config.GetEnv("MONGODB_URI", defaultMongoURI),
		MongoDB:         config.GetEnv("MONGODB_DB", defaultMongoDB),
		MongoCollection: config.GetEnv("MONGODB_COLLECTION", defaultMongoCollection),
		Timeout:         timeout,
		Seed: seed.Config{
			Count:          count,
			RandomSeed:     randomSeed,
			Mode:           seed.Mode(config.GetEnv("SEED_MODE", string(seed.ModeUpsert))),
			SeedVersion:    config.GetEnv("SEED_VERSION", ""),
			TitlePrefix:    config.GetEnv("SEED_TITLE_PREFIX", "Task"),
			Tags:           config.SplitCommaList(config.GetEnv("SEED_TAGS", "demo,seed")),
			DoneRatio:      doneRatio,
			TagCountMin:    tagMin,
			TagCountMax:    tagMax,
			CreatedAtStart: createdAtStart,
			CreatedAtEnd:   createdAtEnd,
			Timeout:        timeout,
		},
	}, nil
}

func parseOptionalTime(key string) (time.Time, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s: %w", key, err)
	}
	return parsed, nil
}
