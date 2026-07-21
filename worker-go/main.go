// Worker do DocPipe: consome document.uploaded, processa o documento e persiste
// o resultado. Fase 0 usa extração mockada — ver internal/processing.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/joaov/docpipe/worker/internal/config"
	"github.com/joaov/docpipe/worker/internal/processing"
	"github.com/joaov/docpipe/worker/internal/queue"
	"github.com/joaov/docpipe/worker/internal/storage"
)

func main() {
	// Log estruturado em JSON: o que a stack de observabilidade da Fase 2 consome.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("worker encerrado com erro", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// SIGTERM (docker stop / scale down) cancela o contexto e dispara o
	// graceful shutdown do consumidor.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	db, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		return err
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return err
	}

	repo := storage.NewRepository(db, rdb)

	consumer, err := queue.NewConsumer(
		cfg.RabbitURL, cfg.Exchange, cfg.Queue, cfg.RoutingKey, cfg.Prefetch)
	if err != nil {
		return err
	}
	defer consumer.Close()

	slog.Info("worker pronto", "queue", cfg.Queue, "prefetch", cfg.Prefetch)

	return consumer.Consume(ctx, func(ctx context.Context, msg queue.DocumentMessage) error {
		return handleDocument(ctx, repo, msg)
	})
}

func handleDocument(
	ctx context.Context, repo *storage.Repository, msg queue.DocumentMessage,
) error {
	started := time.Now()

	log := slog.With("jobId", msg.JobID, "correlationId", msg.CorrelationID)
	log.Info("processando documento", "storageKey", msg.StorageKey)

	if err := repo.SetStatus(ctx, msg.JobID, "extracting"); err != nil {
		return err
	}

	result := processing.Extract(msg.DocumentType)

	if err := repo.SaveResult(
		ctx, msg.JobID, result, result.OverallConfidence, result.NeedsReview,
	); err != nil {
		// Se não conseguimos gravar, marcamos o job como falho em vez de
		// deixá-lo preso em "extracting" para sempre.
		if failErr := repo.Fail(ctx, msg.JobID, err.Error()); failErr != nil {
			log.Error("falha ao marcar job como failed", "error", failErr)
		}
		return err
	}

	log.Info("documento processado",
		"confidence", result.OverallConfidence,
		"needsReview", result.NeedsReview,
		"durationMs", time.Since(started).Milliseconds())

	return nil
}
