namespace DocPipe.Domain;

/// <summary>
/// Estados possíveis de um job. Os valores em string são o contrato compartilhado
/// com o worker Go e com a coluna <c>jobs.status</c> do Postgres.
/// </summary>
public static class JobStatus
{
    public const string Queued = "queued";
    public const string Preprocessing = "preprocessing";
    public const string Extracting = "extracting";
    public const string Completed = "completed";
    public const string Failed = "failed";
    public const string NeedsReview = "needs_review";

    public static readonly IReadOnlySet<string> All = new HashSet<string>
    {
        Queued, Preprocessing, Extracting, Completed, Failed, NeedsReview
    };

    public static bool IsTerminal(string status) =>
        status is Completed or Failed or NeedsReview;
}
