package config

import "os"

type Config struct {
	Port      string
	DBDsn     string
	JWTKey    string
	RedisAddr string
}

func Load() *Config {
	return &Config{
		Port:      envDefault("PORT", "18080"),
		DBDsn:     envDefault("DB_DSN", "root:root@tcp(127.0.0.1:3306)/infinite_canvas?charset=utf8mb4&parseTime=True&loc=Local"),
		JWTKey:    envDefault("JWT_KEY", "change-me-in-production"),
		RedisAddr: envDefault("REDIS_ADDR", "127.0.0.1:6379"),
	}
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
