FROM golang:1.25-alpine AS build
WORKDIR /src

# Camada de dependências separada: só refaz o download quando go.mod/go.sum mudam.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/worker .

# Imagem final mínima, sem shell nem gerenciador de pacotes.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/worker /worker
USER nonroot:nonroot
ENTRYPOINT ["/worker"]
