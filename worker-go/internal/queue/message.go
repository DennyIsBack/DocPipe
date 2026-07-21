// Package queue contém o contrato de mensagem e o consumidor do RabbitMQ.
package queue

import "time"

// DocumentMessage espelha DocPipe.Application.Messaging.DocumentMessage (seção 8.2 do PRD).
// Mudar um campo aqui exige mudar o record correspondente em C#.
type DocumentMessage struct {
	MessageID     string    `json:"messageId"`
	JobID         string    `json:"jobId"`
	DocumentID    string    `json:"documentId"`
	StorageKey    string    `json:"storageKey"`
	DocumentType  string    `json:"documentType"`
	Attempt       int       `json:"attempt"`
	CorrelationID string    `json:"correlationId"`
	Timestamp     time.Time `json:"timestamp"`
}
