using DocPipe.Application.Messaging;

namespace DocPipe.Application.Abstractions;

/// <summary>Publica eventos do pipeline no broker.</summary>
public interface IMessagePublisher
{
    /// <summary>Publica com a routing key informada (ex.: <c>document.uploaded</c>).</summary>
    Task PublishAsync(string routingKey, DocumentMessage message, CancellationToken ct = default);
}
