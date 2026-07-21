namespace DocPipe.Infrastructure;

/// <summary>
/// Todas as credenciais chegam por variável de ambiente (ver .env.example).
/// Nada de segredo em appsettings.json — o repositório é público.
/// </summary>
public class StorageOptions
{
    public const string Section = "Storage";

    public string Endpoint { get; set; } = "http://minio:9000";
    public string AccessKey { get; set; } = "";
    public string SecretKey { get; set; } = "";
    public string Bucket { get; set; } = "documents";
}

public class RabbitOptions
{
    public const string Section = "Rabbit";

    public string Host { get; set; } = "rabbitmq";
    public int Port { get; set; } = 5672;
    public string User { get; set; } = "";
    public string Password { get; set; } = "";
    public string Exchange { get; set; } = "docpipe";
}

public class RedisOptions
{
    public const string Section = "Redis";

    public string ConnectionString { get; set; } = "redis:6379";
}
