namespace DocPipe.Domain;

/// <summary>Unidade de trabalho do pipeline: um documento sendo processado.</summary>
public class Job
{
    public Guid Id { get; init; } = Guid.NewGuid();
    public required Guid DocumentId { get; init; }

    public string Status { get; private set; } = JobStatus.Queued;

    /// <summary>Número de tentativas de processamento (usado no retry da Fase 1).</summary>
    public int Attempt { get; private set; }

    public string? Error { get; private set; }
    public DateTimeOffset CreatedAt { get; init; } = DateTimeOffset.UtcNow;
    public DateTimeOffset UpdatedAt { get; private set; } = DateTimeOffset.UtcNow;

    public void TransitionTo(string status)
    {
        if (!JobStatus.All.Contains(status))
            throw new ArgumentException($"Status desconhecido: {status}", nameof(status));

        Status = status;
        UpdatedAt = DateTimeOffset.UtcNow;
    }

    public void Fail(string error)
    {
        Error = error;
        TransitionTo(JobStatus.Failed);
    }

    public void RegisterAttempt()
    {
        Attempt++;
        UpdatedAt = DateTimeOffset.UtcNow;
    }
}
