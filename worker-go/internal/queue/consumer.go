package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Handler processa uma mensagem. Erro devolvido = falha transitória: a mensagem
// vai para a fila de espera e volta depois. Para falhas definitivas, devolva
// um erro embrulhado com ErrPermanent para mandar direto à DLQ.
type Handler func(ctx context.Context, msg DocumentMessage) error

// ErrPermanent marca o erro que não melhora com retry (documento corrompido,
// tipo não suportado). Use com fmt.Errorf("...: %w", ErrPermanent).
var ErrPermanent = errors.New("falha permanente")

// OnDeadLetter é chamado quando a mensagem esgota as tentativas ou falha em
// definitivo. É o gancho para marcar o job como failed — enquanto há retry
// pendente o job continua no status de processamento, não em failed.
type OnDeadLetter func(ctx context.Context, msg DocumentMessage, cause error)

type Consumer struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	publishCh  *amqp.Channel
	publishMu  sync.Mutex
	queue      string
	exchange   string
	routingKey string

	// OnDeadLetter é opcional; defina antes de chamar Consume.
	OnDeadLetter OnDeadLetter
}

// NewConsumer conecta, declara a topologia completa (trabalho + retry + DLQ) e
// configura o prefetch.
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

	if err := declareTopology(ch, exchange, queue, routingKey); err != nil {
		conn.Close()
		return nil, err
	}

	// Prefetch limita quantas mensagens ficam em voo por worker — é o que
	// distribui a carga de forma justa quando há várias réplicas.
	if err := ch.Qos(prefetch, 0, false); err != nil {
		conn.Close()
		return nil, fmt.Errorf("configurar qos: %w", err)
	}

	// Canal separado para republicar: misturar publish no canal de consumo
	// arrisca fechar o consumo inteiro se uma publicação der erro de canal.
	pub, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("abrir canal de publicacao: %w", err)
	}

	return &Consumer{
		conn:       conn,
		channel:    ch,
		publishCh:  pub,
		queue:      queue,
		exchange:   exchange,
		routingKey: routingKey,
	}, nil
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
		// Mensagem malformada não melhora com retry, mas também não pode
		// evaporar: vai direto para a DLQ.
		slog.Error("mensagem invalida, enviando para a dlq", "error", err)
		c.sendRaw(DLQExchange, c.routingKey, d.Body)
		_ = d.Ack(false)
		return
	}

	if msg.Attempt < 1 {
		msg.Attempt = 1
	}

	log := slog.With(
		"jobId", msg.JobID,
		"correlationId", msg.CorrelationID,
		"attempt", msg.Attempt)

	// O contexto de processamento não é cancelado junto com o shutdown:
	// a mensagem já foi puxada, então terminamos o trabalho antes de sair.
	err := handler(context.WithoutCancel(ctx), msg)
	if err == nil {
		if ackErr := d.Ack(false); ackErr != nil {
			log.Error("falha no ack", "error", ackErr)
		}
		return
	}

	permanent := errors.Is(err, ErrPermanent)
	exhausted := msg.Attempt >= MaxAttempts

	switch {
	case permanent || exhausted:
		log.Error("mensagem enviada para a dlq",
			"error", err, "permanente", permanent, "tentativasEsgotadas", exhausted)
		c.send(DLQExchange, c.routingKey, msg)

		if c.OnDeadLetter != nil {
			c.OnDeadLetter(context.WithoutCancel(ctx), msg, err)
		}

	default:
		tier := tierFor(msg.Attempt)
		msg.Attempt++
		log.Warn("reenfileirando com backoff",
			"error", err, "atraso", backoffTiers[tier].String())
		c.send(RetryExchange, retryQueueName(c.queue, tier), msg)
	}

	// Ack sempre: a responsabilidade pela mensagem já foi transferida para a
	// fila de espera ou para a DLQ. Um nack aqui a devolveria à fila de
	// trabalho na hora, virando um loop quente de reprocessamento.
	if ackErr := d.Ack(false); ackErr != nil {
		log.Error("falha no ack", "error", ackErr)
	}
}

func (c *Consumer) send(exchange, routingKey string, msg DocumentMessage) {
	body, err := json.Marshal(msg)
	if err != nil {
		slog.Error("serializar mensagem para reenvio", "error", err)
		return
	}
	c.sendRaw(exchange, routingKey, body)
}

func (c *Consumer) sendRaw(exchange, routingKey string, body []byte) {
	c.publishMu.Lock()
	defer c.publishMu.Unlock()

	if err := c.publishCh.PublishWithContext(
		context.Background(), exchange, routingKey, false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		}); err != nil {
		slog.Error("republicar mensagem", "exchange", exchange, "error", err)
	}
}

func (c *Consumer) Close() {
	_ = c.publishCh.Close()
	_ = c.channel.Close()
	_ = c.conn.Close()
}
