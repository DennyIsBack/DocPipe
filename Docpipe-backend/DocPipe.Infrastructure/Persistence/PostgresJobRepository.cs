using Dapper;
using DocPipe.Application.Abstractions;
using DocPipe.Domain;
using Npgsql;

namespace DocPipe.Infrastructure.Persistence;

/// <summary>
/// Acesso a dados com SQL cru (Dapper). O schema é versionado em db/migrations e
/// compartilhado com o worker Go, então manter o SQL explícito evita divergência.
/// </summary>
public class PostgresJobRepository : IJobRepository
{
    private readonly NpgsqlDataSource _dataSource;

    public PostgresJobRepository(NpgsqlDataSource dataSource) => _dataSource = dataSource;

    public async Task CreateAsync(Document document, Job job, CancellationToken ct = default)
    {
        await using var conn = await _dataSource.OpenConnectionAsync(ct);
        await using var tx = await conn.BeginTransactionAsync(ct);

        await conn.ExecuteAsync(new CommandDefinition(
            """
            INSERT INTO documents (id, storage_key, original_filename, content_type,
                                   size_bytes, sha256, document_type, created_at)
            VALUES (@Id, @StorageKey, @OriginalFilename, @ContentType,
                    @SizeBytes, @Sha256, @DocumentType, @CreatedAt)
            """,
            document, tx, cancellationToken: ct));

        await conn.ExecuteAsync(new CommandDefinition(
            """
            INSERT INTO jobs (id, document_id, status, attempt, created_at, updated_at)
            VALUES (@Id, @DocumentId, @Status, @Attempt, @CreatedAt, @UpdatedAt)
            """,
            new { job.Id, job.DocumentId, job.Status, job.Attempt, job.CreatedAt, job.UpdatedAt },
            tx, cancellationToken: ct));

        await conn.ExecuteAsync(new CommandDefinition(
            """
            INSERT INTO processing_events (id, job_id, event_type, detail_json)
            VALUES (@Id, @JobId, 'job.created', NULL)
            """,
            new { Id = Guid.NewGuid(), JobId = job.Id }, tx, cancellationToken: ct));

        await tx.CommitAsync(ct);
    }

    public async Task<Job?> GetJobAsync(Guid jobId, CancellationToken ct = default)
    {
        await using var conn = await _dataSource.OpenConnectionAsync(ct);

        var row = await conn.QuerySingleOrDefaultAsync<JobRow>(new CommandDefinition(
            """
            SELECT id, document_id, status, attempt, error, created_at, updated_at
            FROM jobs WHERE id = @jobId
            """,
            new { jobId }, cancellationToken: ct));

        return row?.ToDomain();
    }

    public async Task<ExtractionResult?> GetResultAsync(Guid jobId, CancellationToken ct = default)
    {
        await using var conn = await _dataSource.OpenConnectionAsync(ct);

        var row = await conn.QuerySingleOrDefaultAsync<ResultRow>(new CommandDefinition(
            """
            SELECT id, job_id, payload_json, overall_confidence, created_at
            FROM extraction_results WHERE job_id = @jobId
            """,
            new { jobId }, cancellationToken: ct));

        return row is null
            ? null
            : new ExtractionResult
            {
                Id = row.Id,
                JobId = row.Job_Id,
                PayloadJson = row.Payload_Json,
                OverallConfidence = row.Overall_Confidence,
                CreatedAt = row.Created_At
            };
    }

    public async Task<IReadOnlyList<Job>> ListByStatusAsync(
        string status, int limit, CancellationToken ct = default)
    {
        await using var conn = await _dataSource.OpenConnectionAsync(ct);

        var rows = await conn.QueryAsync<JobRow>(new CommandDefinition(
            """
            SELECT id, document_id, status, attempt, error, created_at, updated_at
            FROM jobs WHERE status = @status
            ORDER BY created_at DESC
            LIMIT @limit
            """,
            new { status, limit }, cancellationToken: ct));

        return rows.Select(r => r.ToDomain()).ToList();
    }

    // Dapper mapeia snake_case → snake_case; a conversão para o domínio fica explícita aqui.
    private sealed record JobRow(
        Guid Id, Guid Document_Id, string Status, int Attempt,
        string? Error, DateTimeOffset Created_At, DateTimeOffset Updated_At)
    {
        public Job ToDomain() =>
            Job.Restore(Id, Document_Id, Status, Attempt, Error, Created_At, Updated_At);
    }

    private sealed record ResultRow(
        Guid Id, Guid Job_Id, string Payload_Json,
        decimal Overall_Confidence, DateTimeOffset Created_At);
}
