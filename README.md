# DocPipe

Pipeline assíncrono de extração inteligente de documentos (notas fiscais, recibos, faturas).
Upload → fila → workers → JSON estruturado com score de confiança.

> **Status:** Fase 0 em construção — fluxo assíncrono ponta-a-ponta com extração mockada.

## Stack

| Serviço | Linguagem | Responsabilidade |
|---|---|---|
| API Gateway | C# / ASP.NET Core | Auth, upload, publicação na fila, status e resultado |
| Worker | Go | Consome a fila, processa o documento, persiste e atualiza o cache |
| Extraction | Python *(Fase 1)* | OCR + parsing de layout + score de confiança |

Infra: RabbitMQ · Redis · PostgreSQL · MinIO · Docker Compose

## Rodando local

```bash
cp .env.example .env   # ajuste as senhas
docker compose up --build
```

A seção completa de arquitetura e as decisões de stack entram no commit final da Fase 0.
