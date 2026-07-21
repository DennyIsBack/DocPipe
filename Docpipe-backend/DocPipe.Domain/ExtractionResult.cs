namespace DocPipe.Domain;

/// <summary>
/// Resultado da extração de um job. O payload fica como JSON cru porque o
/// conjunto de campos varia por tipo de documento — o formato está na seção 8.3 do PRD.
/// </summary>
public class ExtractionResult
{
    /// <summary>Abaixo disso o documento vai para revisão humana.</summary>
    public const decimal ReviewThreshold = 0.85m;

    public Guid Id { get; init; } = Guid.NewGuid();
    public required Guid JobId { get; init; }
    public required string PayloadJson { get; init; }
    public required decimal OverallConfidence { get; init; }
    public DateTimeOffset CreatedAt { get; init; } = DateTimeOffset.UtcNow;

    public bool NeedsReview => OverallConfidence < ReviewThreshold;
}
