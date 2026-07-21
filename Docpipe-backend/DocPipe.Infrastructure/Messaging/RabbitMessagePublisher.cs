using System.Text;
using System.Text.Json;
using DocPipe.Application.Abstractions;
using DocPipe.Application.Messaging;
using Microsoft.Extensions.Options;
using RabbitMQ.Client;

namespace DocPipe.Infrastructure.Messaging;

/// <summary>
/// Publisher de topic exchange. A conexão é cara e thread-safe, então vive como
/// singleton; o canal é protegido por lock porque IModel não é thread-safe.
/// </summary>
public sealed class RabbitMessagePublisher : IMessagePublisher, IDisposable
{
    private readonly IConnection _connection;
    private readonly IModel _channel;
    private readonly RabbitOptions _options;
    private readonly object _publishLock = new();

    public RabbitMessagePublisher(IOptions<RabbitOptions> options)
    {
        _options = options.Value;

        var factory = new ConnectionFactory
        {
            HostName = _options.Host,
            Port = _options.Port,
            UserName = _options.User,
            Password = _options.Password,
            DispatchConsumersAsync = true,
            AutomaticRecoveryEnabled = true
        };

        _connection = factory.CreateConnection("docpipe-api");
        _channel = _connection.CreateModel();

        _channel.ExchangeDeclare(_options.Exchange, ExchangeType.Topic, durable: true);
    }

    public Task PublishAsync(string routingKey, DocumentMessage message, CancellationToken ct = default)
    {
        var body = Encoding.UTF8.GetBytes(JsonSerializer.Serialize(message));

        lock (_publishLock)
        {
            var props = _channel.CreateBasicProperties();
            props.Persistent = true;               // sobrevive a restart do broker
            props.MessageId = message.MessageId.ToString();
            props.CorrelationId = message.CorrelationId.ToString();
            props.ContentType = "application/json";

            _channel.BasicPublish(_options.Exchange, routingKey, props, body);
        }

        return Task.CompletedTask;
    }

    public void Dispose()
    {
        _channel.Dispose();
        _connection.Dispose();
    }
}
