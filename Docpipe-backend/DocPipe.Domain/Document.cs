namespace DocPipe.Domain;

/// <summary>Arquivo recebido, já persistido no object storage.</summary>
public class Document
{
    public Guid Id { get; init; } = Guid.NewGuid();

    /// <summary>Caminho do arquivo no bucket (ex.: <c>2026/07/{id}.pdf</c>).</summary>
    public required string StorageKey { get; init; }

    public required string OriginalFilename { get; init; }
    public required string ContentType { get; init; }
    public required long SizeBytes { get; init; }

    /// <summary>Hash do conteúdo — base para dedupe/idempotência na Fase 1.</summary>
    public required string Sha256 { get; init; }

    public string DocumentType { get; init; } = "invoice";
    public DateTimeOffset CreatedAt { get; init; } = DateTimeOffset.UtcNow;
}
