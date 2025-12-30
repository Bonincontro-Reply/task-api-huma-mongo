package seed

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mode string

const (
	ModeAppend  Mode = "append"
	ModeReplace Mode = "replace"
	ModeUpsert  Mode = "upsert"
	ModeMaintain Mode = "maintain"
)

const (
	defaultCreatedAtDays  = 30
	seedKeyIndexName      = "seedKey_unique"
	defaultSeedVersionFmt = "seed-%d"
)

type Config struct {
	Count          int
	RandomSeed     int64
	Mode           Mode
	SeedVersion    string
	TitlePrefix    string
	Tags           []string
	DoneRatio      float64
	TagCountMin    int
	TagCountMax    int
	CreatedAtStart time.Time
	CreatedAtEnd   time.Time
	Timeout        time.Duration
}

type Result struct {
	Inserted int64
	Updated  int64
	Deleted  int64
}

func Run(ctx context.Context, collection *mongo.Collection, cfg Config) (Result, error) {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return Result{}, err
	}
	cfg = normalized

	if cfg.Mode == ModeReplace {
		delCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
		res, err := collection.DeleteMany(delCtx, bson.M{})
		if err != nil {
			return Result{}, err
		}
		cfgResult := Result{Deleted: res.DeletedCount}
		seedResult, err := seedDocuments(ctx, collection, cfg)
		if err != nil {
			return Result{}, err
		}
		cfgResult.Inserted = seedResult.Inserted
		cfgResult.Updated = seedResult.Updated
		return cfgResult, nil
	}

	if cfg.Mode == ModeMaintain {
		return maintainDocuments(ctx, collection, cfg)
	}

	return seedDocuments(ctx, collection, cfg)
}

func normalizeConfig(cfg Config) (Config, error) {
	if cfg.CreatedAtStart.IsZero() || cfg.CreatedAtEnd.IsZero() {
		now := time.Now().UTC()
		if cfg.CreatedAtEnd.IsZero() {
			cfg.CreatedAtEnd = now
		}
		if cfg.CreatedAtStart.IsZero() {
			cfg.CreatedAtStart = now.AddDate(0, 0, -defaultCreatedAtDays)
		}
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	cfg.Mode = Mode(strings.ToLower(string(cfg.Mode)))
	switch cfg.Mode {
	case "", ModeAppend:
		cfg.Mode = ModeAppend
	case ModeReplace, ModeUpsert, ModeMaintain:
	default:
		return Config{}, fmt.Errorf("invalid seed mode: %s", cfg.Mode)
	}

	if cfg.Count <= 0 {
		return Config{}, errors.New("seed count must be greater than zero")
	}
	if cfg.DoneRatio < 0 || cfg.DoneRatio > 1 {
		return Config{}, errors.New("seed done ratio must be between 0 and 1")
	}
	if cfg.TagCountMin < 0 || cfg.TagCountMax < 0 {
		return Config{}, errors.New("seed tag counts must be non-negative")
	}
	if cfg.TagCountMax < cfg.TagCountMin {
		return Config{}, errors.New("seed tag max must be >= min")
	}
	if cfg.CreatedAtEnd.Before(cfg.CreatedAtStart) {
		return Config{}, errors.New("createdAt end must be after start")
	}
	return cfg, nil
}

func seedDocuments(ctx context.Context, collection *mongo.Collection, cfg Config) (Result, error) {
	if cfg.Mode == ModeUpsert {
		if err := ensureSeedKeyIndex(ctx, collection, cfg.Timeout); err != nil {
			return Result{}, err
		}
	}

	seedVersion := cfg.SeedVersion
	if seedVersion == "" {
		seedVersion = fmt.Sprintf(defaultSeedVersionFmt, cfg.RandomSeed)
	}

	rnd := rand.New(rand.NewSource(cfg.RandomSeed))
	docs := buildDocs(rnd, cfg, cfg.Count, seedVersion, "", nil)

	switch cfg.Mode {
	case ModeUpsert:
		return upsertDocuments(ctx, collection, cfg.Timeout, docs)
	case ModeAppend:
		return insertDocuments(ctx, collection, cfg.Timeout, docs)
	default:
		return Result{}, fmt.Errorf("unsupported seed mode: %s", cfg.Mode)
	}
}

func maintainDocuments(ctx context.Context, collection *mongo.Collection, cfg Config) (Result, error) {
	delCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	delRes, err := collection.DeleteMany(delCtx, bson.M{"done": true})
	if err != nil {
		return Result{}, err
	}

	countCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	currentCount, err := collection.CountDocuments(countCtx, bson.M{})
	if err != nil {
		return Result{}, err
	}
	if currentCount >= int64(cfg.Count) {
		return Result{Deleted: delRes.DeletedCount}, nil
	}

	insertCount := cfg.Count - int(currentCount)
	if insertCount <= 0 {
		return Result{Deleted: delRes.DeletedCount}, nil
	}

	seedVersion := cfg.SeedVersion
	if seedVersion == "" {
		seedVersion = fmt.Sprintf(defaultSeedVersionFmt, cfg.RandomSeed)
	}

	now := time.Now().UTC()
	runKey := fmt.Sprintf("%d", now.UnixNano())
	rnd := rand.New(rand.NewSource(cfg.RandomSeed + now.UnixNano()))
	docs := buildDocs(rnd, cfg, insertCount, seedVersion, runKey, boolPtr(false))

	insertRes, err := insertDocuments(ctx, collection, cfg.Timeout, docs)
	if err != nil {
		return Result{}, err
	}
	insertRes.Deleted = delRes.DeletedCount
	return insertRes, nil
}

func ensureSeedKeyIndex(ctx context.Context, collection *mongo.Collection, timeout time.Duration) error {
	idx := mongo.IndexModel{
		Keys: bson.D{{Key: "seedKey", Value: 1}},
		Options: options.Index().
			SetName(seedKeyIndexName).
			SetUnique(true).
			SetPartialFilterExpression(bson.M{"seedKey": bson.M{"$exists": true}}),
	}
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := collection.Indexes().CreateOne(opCtx, idx)
	return err
}

func insertDocuments(ctx context.Context, collection *mongo.Collection, timeout time.Duration, docs []bson.M) (Result, error) {
	if len(docs) == 0 {
		return Result{}, nil
	}
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	interfaces := make([]interface{}, 0, len(docs))
	for _, doc := range docs {
		interfaces = append(interfaces, doc)
	}
	res, err := collection.InsertMany(opCtx, interfaces, options.InsertMany().SetOrdered(false))
	if err != nil {
		return Result{}, err
	}
	return Result{Inserted: int64(len(res.InsertedIDs))}, nil
}

func upsertDocuments(ctx context.Context, collection *mongo.Collection, timeout time.Duration, docs []bson.M) (Result, error) {
	if len(docs) == 0 {
		return Result{}, nil
	}
	models := make([]mongo.WriteModel, 0, len(docs))
	for _, doc := range docs {
		filter := bson.M{"seedKey": doc["seedKey"]}
		update := bson.M{
			"$set":         doc,
			"$setOnInsert": bson.M{"_id": primitive.NewObjectID()},
		}
		model := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true)
		models = append(models, model)
	}

	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := collection.BulkWrite(opCtx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return Result{}, err
	}
	return Result{
		Inserted: res.UpsertedCount,
		Updated:  res.ModifiedCount,
	}, nil
}

func buildTitle(rnd *rand.Rand, prefix string, index int) string {
	adj := adjectives[rnd.Intn(len(adjectives))]
	noun := nouns[rnd.Intn(len(nouns))]
	if prefix == "" {
		return fmt.Sprintf("%s %s #%d", adj, noun, index+1)
	}
	return fmt.Sprintf("%s %s %s #%d", prefix, adj, noun, index+1)
}

func buildDocs(rnd *rand.Rand, cfg Config, count int, seedVersion, seedKeyPrefix string, doneOverride *bool) []bson.M {
	now := time.Now().UTC()
	docs := make([]bson.M, 0, count)
	for i := 0; i < count; i++ {
		title := buildTitle(rnd, cfg.TitlePrefix, i)
		doneValue := rnd.Float64() < cfg.DoneRatio
		if doneOverride != nil {
			doneValue = *doneOverride
		}
		doc := bson.M{
			"title":       title,
			"done":        doneValue,
			"tags":        pickTags(rnd, cfg.Tags, cfg.TagCountMin, cfg.TagCountMax),
			"createdAt":   randomTime(rnd, cfg.CreatedAtStart, cfg.CreatedAtEnd),
			"seedKey":     seedKey(seedVersion, seedKeyPrefix, i),
			"seedVersion": seedVersion,
			"seedIndex":   i,
			"seededAt":    now,
		}
		docs = append(docs, doc)
	}
	return docs
}

func boolPtr(value bool) *bool {
	return &value
}

func seedKey(seedVersion, prefix string, index int) string {
	if prefix == "" {
		return fmt.Sprintf("%s-%d", seedVersion, index)
	}
	return fmt.Sprintf("%s-%s-%d", seedVersion, prefix, index)
}

func pickTags(rnd *rand.Rand, pool []string, min, max int) []string {
	if len(pool) == 0 || max == 0 {
		return nil
	}
	if max > len(pool) {
		max = len(pool)
	}
	if min > max {
		min = max
	}
	count := min
	if max > min {
		count = min + rnd.Intn(max-min+1)
	}
	if count == 0 {
		return nil
	}

	perm := rnd.Perm(len(pool))
	out := make([]string, 0, count)
	for _, idx := range perm[:count] {
		out = append(out, pool[idx])
	}
	return out
}

func randomTime(rnd *rand.Rand, start, end time.Time) time.Time {
	start = start.UTC()
	end = end.UTC()
	if !end.After(start) {
		return start
	}
	delta := end.Sub(start)
	offset := time.Duration(rnd.Int63n(int64(delta)))
	return start.Add(offset)
}

var adjectives = []string{
	"Quick",
	"Bright",
	"Calm",
	"Brave",
	"Quiet",
	"Fresh",
	"Sharp",
	"Solid",
	"Smart",
	"Kind",
}

var nouns = []string{
	"Report",
	"Review",
	"Call",
	"Plan",
	"Update",
	"Fix",
	"Task",
	"Note",
	"Draft",
	"Deploy",
}
