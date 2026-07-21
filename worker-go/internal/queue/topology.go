package queue

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Topologia de retry com backoff sem depender do delayed-message plugin.
//
// A mensagem que falha é publicada numa fila de espera que só tem TTL e um
// dead-letter apontando de volta para a exchange principal. Quando o TTL expira,
// o próprio broker devolve a mensagem para a fila de trabalho — o atraso sai de
// graça, sem sleep segurando worker nem scheduler externo.
//
//	docpipe ──► document.uploaded ──(falhou)──► docpipe.retry
//	   ▲                                             │
//	   └──────────(TTL expira, dead-letter)──────────┘
//
// Depois de MaxAttempts, a mensagem vai para a DLQ e para de circular:
// nada some, e dá pra inspecionar o que quebrou.
const (
	RetryExchange = "docpipe.retry"
	DLQExchange   = "docpipe.dlq"

	// MaxAttempts inclui a tentativa original.
	MaxAttempts = 4
)

// backoffTiers define o atraso de cada retentativa. Uma fila por degrau: com uma
// fila só e TTL por mensagem, uma mensagem de espera longa na cabeça da fila
// seguraria as de trás (head-of-line blocking).
var backoffTiers = []time.Duration{
	5 * time.Second,
	30 * time.Second,
	2 * time.Minute,
}

func retryQueueName(queue string, tier int) string {
	return fmt.Sprintf("%s.retry.%d", queue, tier)
}

func dlqName(queue string) string {
	return queue + ".dlq"
}

// tierFor escolhe o degrau de backoff pela tentativa que acabou de falhar.
// Tentativas além do último degrau reusam o maior atraso.
func tierFor(attempt int) int {
	tier := attempt - 1
	if tier < 0 {
		return 0
	}
	if tier >= len(backoffTiers) {
		return len(backoffTiers) - 1
	}
	return tier
}

// declareTopology cria exchanges, filas e bindings. Tudo idempotente: API e
// worker podem subir em qualquer ordem, e um restart não quebra nada.
func declareTopology(ch *amqp.Channel, exchange, queue, routingKey string) error {
	for _, ex := range []string{exchange, RetryExchange, DLQExchange} {
		if err := ch.ExchangeDeclare(ex, "topic", true, false, false, false, nil); err != nil {
			return fmt.Errorf("declarar exchange %s: %w", ex, err)
		}
	}

	// Fila de trabalho. O dead-letter aqui pega só o que o broker rejeita
	// sozinho (ex.: fila cheia); a falha de processamento é roteada no código,
	// porque precisamos incrementar o attempt antes.
	if _, err := ch.QueueDeclare(queue, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    DLQExchange,
		"x-dead-letter-routing-key": routingKey,
	}); err != nil {
		return fmt.Errorf("declarar fila %s: %w", queue, err)
	}

	if err := ch.QueueBind(queue, routingKey, exchange, false, nil); err != nil {
		return fmt.Errorf("bind da fila %s: %w", queue, err)
	}

	// Filas de espera: sem consumidor, só TTL + volta para a exchange principal.
	for tier, delay := range backoffTiers {
		name := retryQueueName(queue, tier)

		if _, err := ch.QueueDeclare(name, true, false, false, false, amqp.Table{
			"x-message-ttl":             int32(delay.Milliseconds()),
			"x-dead-letter-exchange":    exchange,
			"x-dead-letter-routing-key": routingKey,
		}); err != nil {
			return fmt.Errorf("declarar fila de retry %s: %w", name, err)
		}

		if err := ch.QueueBind(name, name, RetryExchange, false, nil); err != nil {
			return fmt.Errorf("bind da fila de retry %s: %w", name, err)
		}
	}

	// DLQ: destino final, inspecionado à mão.
	dlq := dlqName(queue)
	if _, err := ch.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declarar dlq %s: %w", dlq, err)
	}

	if err := ch.QueueBind(dlq, routingKey, DLQExchange, false, nil); err != nil {
		return fmt.Errorf("bind da dlq %s: %w", dlq, err)
	}

	return nil
}
