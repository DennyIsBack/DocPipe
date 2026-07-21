using System.Security.Cryptography;
using System.Text;

namespace DocPipe.Api.Auth;

/// <summary>
/// Auth por header <c>X-API-Key</c>. Fase 0: chave única vinda de variável de ambiente.
/// Fase 1: chaves hasheadas na tabela api_keys, com emissão e revogação.
/// </summary>
public class ApiKeyMiddleware
{
    public const string HeaderName = "X-API-Key";

    private static readonly string[] PublicPaths = ["/health", "/ready", "/swagger"];

    private readonly RequestDelegate _next;
    private readonly byte[] _expectedKey;

    public ApiKeyMiddleware(RequestDelegate next, IConfiguration configuration)
    {
        _next = next;

        var key = configuration["ApiKey"];
        if (string.IsNullOrWhiteSpace(key))
            throw new InvalidOperationException("ApiKey não configurada (veja .env.example).");

        _expectedKey = Encoding.UTF8.GetBytes(key);
    }

    public async Task InvokeAsync(HttpContext context)
    {
        var path = context.Request.Path.Value ?? string.Empty;
        if (PublicPaths.Any(p => path.StartsWith(p, StringComparison.OrdinalIgnoreCase)))
        {
            await _next(context);
            return;
        }

        if (!context.Request.Headers.TryGetValue(HeaderName, out var provided) || !IsValid(provided!))
        {
            context.Response.StatusCode = StatusCodes.Status401Unauthorized;
            await context.Response.WriteAsJsonAsync(new { error = "API key inválida ou ausente." });
            return;
        }

        await _next(context);
    }

    // Comparação em tempo constante: evita distinguir chaves por tempo de resposta.
    private bool IsValid(string provided) =>
        CryptographicOperations.FixedTimeEquals(Encoding.UTF8.GetBytes(provided), _expectedKey);
}
