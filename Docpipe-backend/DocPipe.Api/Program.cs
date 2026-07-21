using DocPipe.Api.Auth;
using DocPipe.Application;
using DocPipe.Infrastructure;
using Npgsql;
using StackExchange.Redis;

var builder = WebApplication.CreateBuilder(args);

builder.Services.AddControllers();
builder.Services.AddEndpointsApiExplorer();
builder.Services.AddSwaggerGen();

builder.Services.AddApplication();
builder.Services.AddInfrastructure(builder.Configuration);

var app = builder.Build();

if (app.Environment.IsDevelopment())
{
    app.UseSwagger();
    app.UseSwaggerUI();
}

// Sem UseHttpsRedirection: o TLS termina fora do container.
app.UseMiddleware<ApiKeyMiddleware>();
app.MapControllers();

// Liveness: o processo está de pé.
app.MapGet("/health", () => Results.Ok(new { status = "healthy" }));

// Readiness: as dependências respondem — é o que o compose usa para liberar tráfego.
app.MapGet("/ready", async (NpgsqlDataSource db, IConnectionMultiplexer redis) =>
{
    try
    {
        await using var conn = await db.OpenConnectionAsync();
        await using var cmd = conn.CreateCommand();
        cmd.CommandText = "SELECT 1";
        await cmd.ExecuteScalarAsync();

        await redis.GetDatabase().PingAsync();

        return Results.Ok(new { status = "ready" });
    }
    catch (Exception ex)
    {
        return Results.Json(
            new { status = "not_ready", detail = ex.Message },
            statusCode: StatusCodes.Status503ServiceUnavailable);
    }
});

app.Run();
