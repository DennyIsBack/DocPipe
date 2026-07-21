# DocPipe

Pipeline assíncrono de extração inteligente de documentos (notas fiscais, recibos, faturas).
Você envia um PDF ou a foto de um recibo, recebe um `jobId` na hora, e o dado estruturado
volta como JSON com **score de confiança** por campo.

> **Status:** Fase 0 concluída — fluxo assíncrono ponta-a-ponta rodando local.
> A extração é mockada nesta fase; o OCR real entra na Fase 1.

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

O que já está em pé na Fase 0:

- **Durabilidade** — exchange, filas e mensagens persistentes; o job sobrevive a restart do broker.
- **Ack manual** — a mensagem só sai da fila depois do resultado gravado. Se o worker morre
  no meio, o broker reentrega. Matar o worker durante o processamento não perde documento.
- **Transação única** — resultado, status final e evento de auditoria entram juntos:
  nenhum job fica `completed` sem resultado.
- **Graceful shutdown** — no SIGTERM o worker para de puxar mensagens novas, drena as em voo
  e só então encerra.
- **Cache com fallback** — se o Redis cair, o `GET` de status vai ao Postgres e repovoa o cache.

Fase 1 acrescenta: idempotência por `messageId`, retry com backoff, DLQ e OpenTelemetry.

## Segurança

- Nenhuma credencial no repositório — tudo vem de variável de ambiente (`.env.example`).
- API key comparada em tempo constante.
- Validação de content-type e limite de tamanho no upload.
- Containers rodando como usuário não-root.

## Roadmap

- **Fase 0** ✅ fluxo assíncrono ponta-a-ponta, extração mockada
- **Fase 1** — OCR real em Python, idempotência, retry + DLQ, webhooks assinados, OpenTelemetry
- **Fase 2** — deploy em cloud com Terraform, autoscaling e teste de carga (throughput 1 vs N workers)
