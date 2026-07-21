namespace DocPipe.Application.Abstractions;

/// <summary>Object storage do arquivo original (MinIO local, S3/Blob na cloud).</summary>
public interface IDocumentStorage
{
    /// <summary>Grava o arquivo e devolve a storage key.</summary>
    Task<string> SaveAsync(
        Stream content,
        string storageKey,
        string contentType,
        CancellationToken ct = default);
}
