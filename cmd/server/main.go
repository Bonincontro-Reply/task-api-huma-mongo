package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"task-api-huma-mongo/internal/api"
	"task-api-huma-mongo/internal/service"
	"task-api-huma-mongo/internal/store"
)

const (
	defaultPort            = 8080
	defaultMongoURI        = "mongodb://localhost:27017"
	defaultMongoDB         = "taskdb"
	defaultMongoCollection = "tasks"
	defaultDBTimeout       = 5 * time.Second
)

type Config struct {
	Port             int
	MongoURI         string
	MongoDB          string
	MongoCollection  string
	CORSAllowOrigins []string
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := loadConfig()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	mongoStore, err := store.NewMongoStore(ctx, cfg.MongoURI, cfg.MongoDB, cfg.MongoCollection, defaultDBTimeout)
	if err != nil {
		slog.Error("mongo connect error", "err", err)
		os.Exit(1)
	}
	defer func() {
		_ = mongoStore.Disconnect(context.Background())
	}()

	repo := store.NewMongoTaskRepository(mongoStore)
	svc := service.New(repo)

	mux := http.NewServeMux()
	api.InstallErrorHandler()

	humaAPI := humago.New(mux, huma.DefaultConfig("Task API", "1.0.0"))
	api.RegisterRoutes(humaAPI, svc)

	handler := api.RequestLoggingMiddleware(
		api.CorrelationMiddleware(
			api.CORSMiddleware(cfg.CORSAllowOrigins)(mux),
		),
	)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("server started", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("shutdown started")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}

func loadConfig() (Config, error) {
	portValue := getEnv("PORT", strconv.Itoa(defaultPort))
	port, err := strconv.Atoi(portValue)
	if err != nil || port <= 0 {
		return Config{}, fmt.Errorf("invalid PORT: %s", portValue)
	}

	return Config{
		Port:             port,
		MongoURI:         getEnv("MONGODB_URI", defaultMongoURI),
		MongoDB:          getEnv("MONGODB_DB", defaultMongoDB),
		MongoCollection:  getEnv("MONGODB_COLLECTION", defaultMongoCollection),
		CORSAllowOrigins: splitCommaList(getEnv("CORS_ALLOW_ORIGINS", "http://localhost:8081,http://127.0.0.1:8081")),
	}, nil
}

func getEnv(key, defValue string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return defValue
	}
	return value
}

func splitCommaList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
