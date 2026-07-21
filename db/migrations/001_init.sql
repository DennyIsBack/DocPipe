-- DocPipe — schema inicial (Fase 0)
-- api_keys e webhooks entram na Fase 1.

CREATE TABLE IF NOT EXISTS documents (
    id                UUID PRIMARY KEY,
    storage_key       TEXT        NOT NULL,
    original_filename TEXT        NOT NULL,
    content_type      TEXT        NOT NULL,
    size_bytes        BIGINT      NOT NULL,
    sha256            CHAR(64)    NOT NULL,
    document_type     TEXT        NOT NULL DEFAULT 'invoice',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_documents_sha256 ON documents (sha256);

CREATE TABLE IF NOT EXISTS jobs (
    id          UUID PRIMARY KEY,
    document_id UUID        NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    status      TEXT        NOT NULL DEFAULT 'queued'
                CHECK (status IN ('queued', 'preprocessing', 'extracting',
                                  'completed', 'failed', 'needs_review')),
    attempt     INT         NOT NULL DEFAULT 0,
    error       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_jobs_status     ON jobs (status);
CREATE INDEX IF NOT EXISTS ix_jobs_created_at ON jobs (created_at DESC);

CREATE TABLE IF NOT EXISTS extraction_results (
    id                 UUID PRIMARY KEY,
    job_id             UUID        NOT NULL UNIQUE REFERENCES jobs (id) ON DELETE CASCADE,
    payload_json       JSONB       NOT NULL,
    overall_confidence NUMERIC(4, 3) NOT NULL,
    needs_review       BOOLEAN     NOT NULL DEFAULT false,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Trilha de auditoria: uma linha por transição/evento do job.
CREATE TABLE IF NOT EXISTS processing_events (
    id          UUID PRIMARY KEY,
    job_id      UUID        NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    event_type  TEXT        NOT NULL,
    detail_json JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_processing_events_job_id ON processing_events (job_id, created_at);
