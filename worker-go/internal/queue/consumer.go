package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Handler processa uma mensagem. Erro = nack; a Fase 1 acrescenta retry com backoff e DLQ.
type Handler func(ctx context.Context, msg DocumentMessage) error

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   string
}

// NewConsumer declara exchange, fila e binding de forma idempotente — o worker
// pode subir antes ou depois da API, em qualquer ordem.
func NewConsumer(url, exchange, queue, routingKey string, prefetch int) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("conectar no rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("abrir canal: %w", err)
	}

	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("declarar exchange: %w", err)
	}

	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("declarar fila: %w", err)
	}

	if err := ch.QueueBind(queue, routingKey, exchange, false, nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("bind da fila: %w", err)
	}

	// Prefetch limita quantas mensagens ficam em voo por worker — é o que
	// distribui a carga de forma justa quando há várias réplicas.
	if err := ch.Qos(prefetch, 0, false); err != nil {
		conn.Close()
		return nil, fmt.Errorf("configurar qos: %w", err)
	}

	return &Consumer{conn: conn, channel: ch, queue: queue}, nil
}

// Consume bloqueia até o contexto ser cancelado. Ao cancelar, para de puxar
// mensagens novas e espera as em voo terminarem antes de retornar — nenhum job
// fica órfão quando o container recebe SIGTERM.
func (c *Consumer) Consume(ctx context.Context, handler Handler) error {
	deliveries, err := c.channel.Consume(c.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("iniciar consumo: %w", err)
	}

	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			slog.Info("drenando mensagens em voo antes de encerrar")
			wg.Wait()
			return nil

		case delivery, ok := <-deliveries:
			if !ok {
				wg.Wait()
				return fmt.Errorf("canal de entregas fechado pelo broker")
			}

			wg.Add(1)
			go func(d amqp.Delivery) {
				defer wg.Done()
				c.handle(ctx, d, handler)
			}(delivery)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, d amqp.Delivery, handler Handler) {
	var msg DocumentMessage
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		// Mensagem malformada não melhora com retry: descarta sem requeue.
		slog.Error("mensagem invalida, descartando", "error", err)
		_ = d.Nack(false, false)
		return
	}

	// O contexto de processamento não é cancelado junto com o shutdown:
	// a mensagem já foi puxada, então terminamos o trabalho antes de sair.
	if err := handler(context.WithoutCancel(ctx), msg); err != nil {
		slog.Error("falha ao processar",
			"jobId", msg.JobID, "correlationId", msg.CorrelationID, "error", err)
		_ = d.Nack(false, false)
		return
	}

	if err := d.Ack(false); err != nil {
		slog.Error("falha no ack", "jobId", msg.JobID, "error", err)
	}
}

func (c *Consumer) Close() {
	_ = c.channel.Close()
	_ = c.conn.Close()
}
