# DocPipe

Pipeline assíncrono de extração inteligente de documentos (notas fiscais, recibos, faturas).
Você envia um PDF ou a foto de um recibo, recebe um `jobId` na hora, e o dado estruturado
volta como JSON com **score de confiança** por campo.

> **Status:** Fase 0 concluída e camada de resiliência da Fase 1 no lugar
> (idempotência, retry com backoff e DLQ). A extração ainda é mockada — o OCR real vem a seguir.

## O problema

PMEs e escritórios de contabilidade recebem documentos em rajada e transcrevem à mão:
lento, caro e sujeito a erro. Em pico de fechamento de mês, a fila estoura.

É exatamente o cenário onde um pipeline síncrono trava — por isso o desenho é
**assíncrono, resiliente e escalável horizontalmente**. Sob carga, a resposta é
subir mais workers, não devolver 500.

## Arquitetura

```
Cliente
  │  POST /v1/documents
  ▼
┌──────────────────────┐   document.uploaded   ┌────────────────────────┐
│  API  (C# / ASP.NET) │ ────────────────────► │  Worker  (Go)          │
│  auth, upload,       │      [RabbitMQ]       │  processa o documento, │
│  publica, status     │                       │  persiste, atualiza    │
└──────┬───────────────┘                       └───────┬────────────────┘
       │ status                                        │
       ▼                                               ▼
   ┌───────┐                                    ┌──────────────┐
   │ Redis │◄───────────────────────────────────│  PostgreSQL  │
   └───────┘   cache de status (GET < 50ms)     │  jobs, docs, │
                                                │  resultados  │
   ┌───────┐                                    └──────────────┘
   │ MinIO │  arquivo original (API S3)
   └───────┘
```

O upload não espera o processamento: a API grava o arquivo, cria o job em `queued`,
publica na fila e responde **202** com o `jobId`. O worker consome, processa e escreve
o resultado; a consulta de status é servida do Redis.

### Por que cada linguagem

Nenhuma peça aqui é enfeite — cada uma existe porque o domínio pede.

| Serviço | Linguagem | Por quê |
|---|---|---|
| **API Gateway** | C# / ASP.NET Core | Camada de borda com auth, validação e contrato REST. Clean architecture em 4 projetos (Api → Application → Domain, com Infrastructure implementando as abstrações). |
| **Worker** | Go | Consumidor concorrente e leve. Goroutines + prefetch dão throughput em I/O pesado, e a imagem distroless sobe em milissegundos — o que torna o autoscaling barato. |
| **Extraction** *(Fase 1)* | Python | OCR e visão computacional. É onde o ecossistema de ML se justifica de verdade. |

## Rodando local

Requisitos: Docker Desktop.

```bash
cp .env.example .env      # ajuste as senhas
docker compose up --build
```

| Serviço | URL |
|---|---|
| API (Swagger) | http://localhost:8080/swagger |
| RabbitMQ (UI) | http://localhost:15672 |
| MinIO (console) | http://localhost:9001 |

### Fluxo completo

```bash
# 1. envia um documento
curl -H "X-API-Key: dev-local-key" \
     -F "file=@recibo.pdf" \
     http://localhost:8080/v1/documents
# → {"jobId":"3f2b..."}

# 2. acompanha o status (queued → extracting → completed | needs_review)
curl -H "X-API-Key: dev-local-key" http://localhost:8080/v1/jobs/3f2b...

# 3. pega o resultado estruturado
curl -H "X-API-Key: dev-local-key" http://localhost:8080/v1/jobs/3f2b.../result

# 4. fila de revisão humana
curl -H "X-API-Key: dev-local-key" "http://localhost:8080/v1/jobs?status=needs_review"
```

### Escalando os workers

```bash
docker compose up --scale worker=4
```

## Endpoints

| Método | Rota | Descrição |
|---|---|---|
| `POST` | `/v1/documents` | Upload multipart. Valida tipo e tamanho, devolve `jobId` (202). |
| `GET` | `/v1/jobs/{id}` | Status do job — servido do cache. |
| `GET` | `/v1/jobs/{id}/result` | Resultado da extração. 409 enquanto não terminou. |
| `GET` | `/v1/jobs?status=` | Lista por status (padrão: `needs_review`). |
| `GET` | `/health` `/ready` | Liveness e readiness. |

Auth via header `X-API-Key`.

## Resiliência

- **Durabilidade** — exchange, filas e mensagens persistentes; o job sobrevive a restart do broker.
- **Ack manual** — a mensagem só sai da fila depois do resultado gravado. Se o worker morre
  no meio, o broker reentrega. Matar o worker durante o processamento não perde documento.
- **Idempotência** — entrega ao menos uma vez é garantia do broker, não processamento único.
  Cada mensagem é reservada por `messageId` com `SET NX` no Redis (atômico, então duas réplicas
  não processam a mesma em paralelo), com o status terminal do job como segunda linha de defesa.
- **Retry com backoff** — falha transitória volta para a fila depois de 5s, 30s e 2min.
- **Dead-letter queue** — esgotadas as 4 tentativas, a mensagem para na DLQ e o job vira `failed`.
  Nada some silenciosamente.
- **Transação única** — resultado, status final e evento de auditoria entram juntos:
  nenhum job fica `completed` sem resultado.
- **Graceful shutdown** — no SIGTERM o worker para de puxar mensagens novas, drena as em voo
  e só então encerra.
- **Cache com fallback** — se o Redis cair, o `GET` de status vai ao Postgres e repovoa o cache.

### Como o backoff funciona sem plugin

```
docpipe ──► document.uploaded ──(falhou)──► docpipe.retry ──► [fila com TTL]
   ▲                                                               │
   └───────────────(TTL expira, dead-letter devolve)───────────────┘
```

A mensagem que falha vai para uma fila **sem consumidor**, que só tem TTL e um dead-letter
apontando de volta para a exchange principal. Quando o TTL expira, o próprio broker devolve
a mensagem — o atraso sai de graça, sem `sleep` segurando worker e sem scheduler externo.

É uma fila por degrau de atraso, não uma só com TTL por mensagem: com fila única, uma mensagem
de espera longa na cabeça seguraria todas as de trás (*head-of-line blocking*).

### Vendo funcionar

```bash
# perda zero: mata o worker no meio do processamento
docker compose stop worker      # com jobs em voo
docker compose start worker     # os jobs completam — o broker reentregou

# inspecionar o que foi para a DLQ
# RabbitMQ UI → Queues → document.uploaded.dlq
```

Fase 1 ainda acrescenta: OCR real em Python, webhooks assinados e OpenTelemetry.

## Segurança

- Nenhuma credencial no repositório — tudo vem de variável de ambiente (`.env.example`).
- API key comparada em tempo constante.
- Validação de content-type e limite de tamanho no upload.
- Containers rodando como usuário não-root.

## Roadmap

- **Fase 0** ✅ fluxo assíncrono ponta-a-ponta, extração mockada
- **Fase 1** 🔨 resiliência (idempotência, retry + DLQ) pronta; falta OCR, webhooks e OpenTelemetry
- **Fase 2** — deploy em cloud com Terraform, autoscaling e teste de carga (throughput 1 vs N workers)
