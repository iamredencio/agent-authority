package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iamredencio/agent-authority/internal/decision"
	"github.com/iamredencio/agent-authority/internal/policy"
	"github.com/iamredencio/agent-authority/internal/postgres"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	tp, err := newTracer()
	if err != nil {
		log.Fatalf("tracer: %v", err)
	}
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)

	store, err := postgres.New(ctx, dsn)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	pol, err := loadPolicy(os.Getenv("POLICY_PATH"))
	if err != nil {
		log.Fatalf("policy: %v", err)
	}

	engine := &decision.Engine{
		Store:  store,
		Policy: pol,
		Tracer: tp.Tracer("agent-authority/decision"),
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           decision.Handler(engine),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("decision API listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func loadPolicy(path string) (*policy.Engine, error) {
	if path == "" {
		return policy.Default(), nil
	}
	module, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return policy.New(path, string(module))
}

func newTracer() (*sdktrace.TracerProvider, error) {
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", "agent-authority-decision"),
	))
	if err != nil {
		return nil, err
	}
	return sdktrace.NewTracerProvider(sdktrace.WithResource(res)), nil
}
