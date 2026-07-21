using Amazon.Runtime;
using Amazon.S3;
using DocPipe.Application.Abstractions;
using DocPipe.Infrastructure.Caching;
using DocPipe.Infrastructure.Messaging;
using DocPipe.Infrastructure.Persistence;
using DocPipe.Infrastructure.Storage;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Npgsql;
using StackExchange.Redis;

namespace DocPipe.Infrastructure;

public static class DependencyInjection
{
    public static IServiceCollection AddInfrastructure(
        this IServiceCollection services, IConfiguration configuration)
    {
        services.Configure<StorageOptions>(configuration.GetSection(StorageOptions.Section));
        services.Configure<RabbitOptions>(configuration.GetSection(RabbitOptions.Section));
        services.Configure<RedisOptions>(configuration.GetSection(RedisOptions.Section));

        // Postgres
        var postgres = configuration.GetConnectionString("Postgres")
            ?? throw new InvalidOperationException(
                "ConnectionStrings:Postgres não configurado (veja .env.example).");
        services.AddSingleton(NpgsqlDataSource.Create(postgres));
        services.AddScoped<IJobRepository, PostgresJobRepository>();

        // Redis
        var redis = configuration.GetSection(RedisOptions.Section).Get<RedisOptions>()
            ?? new RedisOptions();
        services.AddSingleton<IConnectionMultiplexer>(
            ConnectionMultiplexer.Connect(redis.ConnectionString));
        services.AddScoped<IJobCache, RedisJobCache>();

        // MinIO / S3
        var storage = configuration.GetSection(StorageOptions.Section).Get<StorageOptions>()
            ?? new StorageOptions();
        services.AddSingleton<IAmazonS3>(_ => new AmazonS3Client(
            new BasicAWSCredentials(storage.AccessKey, storage.SecretKey),
            new AmazonS3Config
            {
                ServiceURL = storage.Endpoint,
                ForcePathStyle = true,   // exigido pelo MinIO
                AuthenticationRegion = "us-east-1"
            }));
        services.AddScoped<IDocumentStorage, S3DocumentStorage>();

        // RabbitMQ — conexão cara, mantida como singleton.
        services.AddSingleton<IMessagePublisher, RabbitMessagePublisher>();

        return services;
    }
}
