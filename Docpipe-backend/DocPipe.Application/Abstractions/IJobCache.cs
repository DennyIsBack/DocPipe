namespace DocPipe.Application.Abstractions;

/// <summary>
/// Cache de status do job no Redis — é o que faz o <c>GET /v1/jobs/{id}</c> responder
/// em menos de 50ms sem tocar no Postgres. O worker Go escreve nas mesmas chaves.
/// </summary>
public interface IJobCache
{
    Task<string?> GetStatusAsync(Guid jobId, CancellationToken ct = default);

    Task SetStatusAsync(Guid jobId, string status, CancellationToken ct = default);
}
