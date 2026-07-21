// Package config carrega a configuração do worker a partir do ambiente.
// Toda credencial vem de variável de ambiente — ver .env.example na raiz.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	PostgresURL string
	RabbitURL   string
	RedisAddr   string
	Exchange    string
	Queue       string
	RoutingKey  string
	Prefetch    int
}

func Load() (Config, error) {
	cfg := Config{
		PostgresURL: os.Getenv("POSTGRES_URL"),
		RabbitURL:   os.Getenv("RABBITMQ_URL"),
		RedisAddr:   getEnv("REDIS_ADDR", "redis:6379"),
		Exchange:    getEnv("RABBITMQ_EXCHANGE", "docpipe"),
		Queue:       getEnv("RABBITMQ_QUEUE", "document.uploaded"),
		RoutingKey:  getEnv("RABBITMQ_ROUTING_KEY", "document.uploaded"),
		Prefetch:    getEnvInt("WORKER_PREFETCH", 4),
	}

	if cfg.PostgresURL == "" {
		return cfg, fmt.Errorf("POSTGRES_URL nao configurada")
	}
	if cfg.RabbitURL == "" {
		return cfg, fmt.Errorf("RABBITMQ_URL nao configurada")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return fallback
}
