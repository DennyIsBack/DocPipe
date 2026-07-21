using DocPipe.Application.Abstractions;
using StackExchange.Redis;

namespace DocPipe.Infrastructure.Caching;

public class RedisJobCache : IJobCache
{
    /// <summary>Formato compartilhado com o worker Go — mudar aqui exige mudar lá.</summary>
    public static string StatusKey(Guid jobId) => $"job:{jobId}:status";

    private static readonly TimeSpan Ttl = TimeSpan.FromHours(24);

    private readonly IConnectionMultiplexer _redis;

    public RedisJobCache(IConnectionMultiplexer redis) => _redis = redis;

    public async Task<string?> GetStatusAsync(Guid jobId, CancellationToken ct = default)
    {
        var value = await _redis.GetDatabase().StringGetAsync(StatusKey(jobId));
        return value.HasValue ? value.ToString() : null;
    }

    public Task SetStatusAsync(Guid jobId, string status, CancellationToken ct = default) =>
        _redis.GetDatabase().StringSetAsync(StatusKey(jobId), status, Ttl);
}
