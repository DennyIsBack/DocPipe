// Package processing contém a extração de dados do documento.
//
// FASE 0: extração MOCKADA. O objetivo desta fase é provar o fluxo assíncrono
// ponta-a-ponta, não a qualidade do OCR. Na Fase 1 este pacote vira um publisher
// para a fila do serviço Python (Tesseract/PaddleOCR) e some daqui.
package processing

import (
	"math/rand"
	"time"
)

type Field struct {
	Value      any     `json:"value"`
	Confidence float64 `json:"confidence"`
}

type Result struct {
	DocumentType      string           `json:"documentType"`
	OverallConfidence float64          `json:"overallConfidence"`
	Fields            map[string]Field `json:"fields"`
	NeedsReview       bool             `json:"needsReview"`
}

// ReviewThreshold espelha ExtractionResult.ReviewThreshold no lado C#.
const ReviewThreshold = 0.85

// Extract devolve um resultado sintético no formato da seção 8.3 do PRD.
// A confiança varia de propósito para exercitar o caminho de needs_review.
func Extract(documentType string) Result {
	// Simula o custo real de OCR — sem isso a fila drena rápido demais
	// para dar pra observar o pipeline funcionando.
	time.Sleep(time.Duration(500+rand.Intn(1500)) * time.Millisecond) //nolint:gosec // mock

	fields := map[string]Field{
		"vendorName":    {Value: "Fornecedor Exemplo LTDA", Confidence: round(0.85 + rand.Float64()*0.14)},
		"invoiceNumber": {Value: "NF-00012345", Confidence: round(0.60 + rand.Float64()*0.39)},
		"issueDate":     {Value: "2026-07-15", Confidence: round(0.80 + rand.Float64()*0.19)},
		"totalAmount":   {Value: 1499.90, Confidence: round(0.88 + rand.Float64()*0.11)},
	}

	var sum float64
	for _, f := range fields {
		sum += f.Confidence
	}
	overall := round(sum / float64(len(fields)))

	return Result{
		DocumentType:      documentType,
		OverallConfidence: overall,
		Fields:            fields,
		NeedsReview:       overall < ReviewThreshold,
	}
}

func round(v float64) float64 {
	return float64(int(v*1000)) / 1000
}
