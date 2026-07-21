using System.Text.Json.Serialization;

namespace DocPipe.Application.Messaging;

/// <summary>
/// Contrato das mensagens do pipeline (seção 8.2 do PRD). Compartilhado com o worker Go —
/// mudar um campo aqui exige mudar o struct correspondente em <c>worker-go</c>.
/// </summary>
public record DocumentMessage
{
    [JsonPropertyName("messageId")]
    public Guid MessageId { get; init; } = Guid.NewGuid();

    [JsonPropertyName("jobId")]
    public required Guid JobId { get; init; }

    [JsonPropertyName("documentId")]
    public required Guid DocumentId { get; init; }

    [JsonPropertyName("storageKey")]
    public required string StorageKey { get; init; }

    [JsonPropertyName("documentType")]
    public required string DocumentType { get; init; }

    [JsonPropertyName("attempt")]
    public int Attempt { get; init; } = 1;

    /// <summary>Atravessa todos os serviços para amarrar o trace ponta-a-ponta.</summary>
    [JsonPropertyName("correlationId")]
    public required Guid CorrelationId { get; init; }

    [JsonPropertyName("timestamp")]
    public DateTimeOffset Timestamp { get; init; } = DateTimeOffset.UtcNow;
}
