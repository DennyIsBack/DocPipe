using DocPipe.Domain;

namespace DocPipe.Application.Abstractions;

public interface IJobRepository
{
    /// <summary>Insere documento e job na mesma transação.</summary>
    Task CreateAsync(Document document, Job job, CancellationToken ct = default);

    Task<Job?> GetJobAsync(Guid jobId, CancellationToken ct = default);

    Task<ExtractionResult?> GetResultAsync(Guid jobId, CancellationToken ct = default);

    Task<IReadOnlyList<Job>> ListByStatusAsync(
        string status, int limit, CancellationToken ct = default);
}
