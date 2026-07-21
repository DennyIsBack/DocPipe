package storage

import (
	"context"
	"fmt"
	"time"
)

// Entrega ao menos uma vez é garantia do broker: um worker que morre depois de
// processar mas antes do ack faz a mensagem ser reentregue. A chave de
// idempotência transforma isso em processamento efetivamente único.
//
// A janela cobre reentrega por crash e por retry, não a vida toda do job —
// reenviar o mesmo documento amanhã é um job novo, de propósito.
const idempotencyTTL = 48 * time.Hour

func idempotencyKey(messageID string) string {
	return fmt.Sprintf("idem:%s", messageID)
}

// ClaimMessage reserva o processamento da mensagem. Devolve false se outra
// entrega já a reservou — nesse caso a duplicata deve ser descartada.
//
// SET NX é atômico no Redis, então duas réplicas que recebam a mesma mensagem
// ao mesmo tempo não conseguem processá-la em paralelo.
func (r *Repository) ClaimMessage(ctx context.Context, messageID string) (bool, error) {
	ok, err := r.cache.SetNX(ctx, idempotencyKey(messageID), "1", idempotencyTTL).Result()
	if err != nil {
		return false, fmt.Errorf("reservar mensagem: %w", err)
	}
	return ok, nil
}

// ReleaseMessage devolve a reserva. Chamado quando o processamento falha e a
// mensagem vai ser retentada: sem isso, a retentativa se veria como duplicata
// e o job ficaria preso sem nunca completar.
func (r *Repository) ReleaseMessage(ctx context.Context, messageID string) error {
	return r.cache.Del(ctx, idempotencyKey(messageID)).Err()
}

// IsJobFinished diz se o job já chegou a um estado terminal. É a segunda linha
// de defesa: se a chave de idempotência expirou mas o trabalho já foi feito,
// não reprocessamos.
func (r *Repository) IsJobFinished(ctx context.Context, jobID string) (bool, error) {
	var status string
	err := r.db.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1`, jobID).Scan(&status)
	if err != nil {
		return false, fmt.Errorf("consultar status do job: %w", err)
	}

	return status == "completed" || status == "needs_review" || status == "failed", nil
}
