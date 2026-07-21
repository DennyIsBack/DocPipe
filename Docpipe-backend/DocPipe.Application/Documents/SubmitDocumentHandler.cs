using System.Security.Cryptography;
using DocPipe.Application.Abstractions;
using DocPipe.Application.Messaging;
using DocPipe.Domain;

namespace DocPipe.Application.Documents;

public record SubmitDocumentCommand(
    Stream Content,
    string Filename,
    string ContentType,
    long SizeBytes,
    string DocumentType);

/// <summary>
/// Caso de uso do upload: grava no object storage, cria job em <c>queued</c> e publica
/// na fila. Devolve o jobId na hora — o processamento acontece depois, fora do request.
/// </summary>
public class SubmitDocumentHandler
{
    public const string UploadedRoutingKey = "document.uploaded";

    private readonly IDocumentStorage _storage;
    private readonly IJobRepository _repository;
    private readonly IMessagePublisher _publisher;
    private readonly IJobCache _cache;

    public SubmitDocumentHandler(
        IDocumentStorage storage,
        IJobRepository repository,
        IMessagePublisher publisher,
        IJobCache cache)
    {
        _storage = storage;
        _repository = repository;
        _publisher = publisher;
        _cache = cache;
    }

    public async Task<Guid> HandleAsync(SubmitDocumentCommand command, CancellationToken ct = default)
    {
        var sha256 = await ComputeSha256Async(command.Content, ct);
        command.Content.Position = 0;

        var documentId = Guid.NewGuid();
        var extension = Path.GetExtension(command.Filename);
        var storageKey = $"{DateTime.UtcNow:yyyy/MM}/{documentId}{extension}";

        await _storage.SaveAsync(command.Content, storageKey, command.ContentType, ct);

        var document = new Document
        {
            Id = documentId,
            StorageKey = storageKey,
            OriginalFilename = command.Filename,
            ContentType = command.ContentType,
            SizeBytes = command.SizeBytes,
            Sha256 = sha256,
            DocumentType = command.DocumentType
        };

        var job = new Job { DocumentId = documentId };

        await _repository.CreateAsync(document, job, ct);
        await _cache.SetStatusAsync(job.Id, job.Status, ct);

        await _publisher.PublishAsync(UploadedRoutingKey, new DocumentMessage
        {
            JobId = job.Id,
            DocumentId = documentId,
            StorageKey = storageKey,
            DocumentType = command.DocumentType,
            CorrelationId = job.Id
        }, ct);

        return job.Id;
    }

    private static async Task<string> ComputeSha256Async(Stream content, CancellationToken ct)
    {
        using var sha = SHA256.Create();
        var hash = await sha.ComputeHashAsync(content, ct);
        return Convert.ToHexString(hash).ToLowerInvariant();
    }
}
