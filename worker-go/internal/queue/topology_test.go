package queue

import "testing"

func TestTierFor(t *testing.T) {
	cases := []struct {
		name    string
		attempt int
		want    int
	}{
		{"primeira falha usa o menor atraso", 1, 0},
		{"segunda falha sobe um degrau", 2, 1},
		{"terceira falha usa o maior degrau", 3, 2},
		{"alem do ultimo degrau reusa o maior", 9, len(backoffTiers) - 1},
		{"attempt invalido cai no primeiro degrau", 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tierFor(tc.attempt); got != tc.want {
				t.Errorf("tierFor(%d) = %d, esperado %d", tc.attempt, got, tc.want)
			}
		})
	}
}

// Garante que o retry esgota antes de a mensagem circular para sempre.
func TestMaxAttemptsCobreTodosOsDegraus(t *testing.T) {
	if MaxAttempts <= len(backoffTiers) {
		t.Errorf("MaxAttempts (%d) precisa ser maior que os %d degraus de backoff, "+
			"senao o ultimo degrau nunca e usado", MaxAttempts, len(backoffTiers))
	}
}

func TestNomesDaTopologia(t *testing.T) {
	if got := retryQueueName("document.uploaded", 0); got != "document.uploaded.retry.0" {
		t.Errorf("retryQueueName = %q", got)
	}
	if got := dlqName("document.uploaded"); got != "document.uploaded.dlq" {
		t.Errorf("dlqName = %q", got)
	}
}
