// Package storage cuida da persistência do worker: Postgres (verdade) e Redis (cache).
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const cacheTTL = 24 * time.Hour

type Repository struct {
	db    *pgxpool.Pool
	cache *redis.Client
}

func NewRepository(db *pgxpool.Pool, cache *redis.Client) *Repository {
	return &Repository{db: db, cache: cache}
}

// statusKey precisa bater com RedisJobCache.StatusKey no lado C#.
func statusKey(jobID string) string {
	return fmt.Sprintf("job:%s:status", jobID)
}

// SetStatus grava no Postgres e atualiza o cache. O Postgres é a fonte da verdade:
// se o Redis falhar, o job continua correto e a API cai no banco no próximo GET.
func (r *Repository) SetStatus(ctx context.Context, jobID, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE jobs SET status = $1, updated_at = now() WHERE id = $2`,
		status, jobID)
	if err != nil {
		return fmt.Errorf("atualizar status do job: %w", err)
	}

	return r.cache.Set(ctx, statusKey(jobID), status, cacheTTL).Err()
}

// Fail marca o job como falho e registra o motivo.
func (r *Repository) Fail(ctx context.Context, jobID, reason string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE jobs SET status = 'failed', error = $1, updated_at = now() WHERE id = $2`,
		reason, jobID)
	if err != nil {
		return fmt.Errorf("marcar job como failed: %w", err)
	}

	return r.cache.Set(ctx, statusKey(jobID), "failed", cacheTTL).Err()
}

// SaveResult grava o resultado, define o status final e atualiza o cache numa
// única transação — o job nunca fica "completed" sem resultado gravado.
func (r *Repository) SaveResult(
	ctx context.Context, jobID string, payload any, confidence float64, needsReview bool,
) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("serializar payload: %w", err)
	}

	status := "completed"
	if needsReview {
		status = "needs_review"
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("abrir transacao: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op depois do commit

	if _, err = tx.Exec(ctx,
		`INSERT INTO extraction_results
		     (id, job_id, payload_json, overall_confidence, needs_review)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (job_id) DO NOTHING`,
		uuid.New(), jobID, payloadJSON, confidence, needsReview); err != nil {
		return fmt.Errorf("inserir resultado: %w", err)
	}

	if _, err = tx.Exec(ctx,
		`UPDATE jobs SET status = $1, updated_at = now() WHERE id = $2`,
		status, jobID); err != nil {
		return fmt.Errorf("finalizar job: %w", err)
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO processing_events (id, job_id, event_type, detail_json)
		 VALUES ($1, $2, 'job.completed', $3)`,
		uuid.New(), jobID,
		fmt.Sprintf(`{"confidence": %.3f}`, confidence)); err != nil {
		return fmt.Errorf("registrar evento: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return r.cache.Set(ctx, statusKey(jobID), status, cacheTTL).Err()
}
