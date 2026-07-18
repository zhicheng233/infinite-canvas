package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                         string
	DBDsn                        string
	JWTKey                       string
	RedisAddr                    string
	RegistrationCredits          int
	ApiKeyEncryptKey             string
	InitAdminUsername            string
	InitAdminPassword            string
	InitAdminDisplayName         string
	TmpMediaDir                  string
	PublicBaseURL                string
	OnDemandRepairURL            string
	OnDemandRepairUser           string
	OnDemandRepairPass           string
	OnDemandRepairTimeoutSeconds int
}

func Load() *Config {
	return &Config{
		Port:                         envDefault("PORT", "18080"),
		DBDsn:                        envDefault("DB_DSN", "root:root@tcp(127.0.0.1:3306)/infinite_canvas?charset=utf8mb4&parseTime=True&loc=Local"),
		JWTKey:                       envDefault("JWT_KEY", "change-me-in-production"),
		RedisAddr:                    envDefault("REDIS_ADDR", "127.0.0.1:6379"),
		RegistrationCredits:          envDefaultInt("REGISTRATION_CREDITS", 100),
		ApiKeyEncryptKey:             envDefault("API_KEY_ENCRYPTION_KEY", ""),
		InitAdminUsername:            envDefault("INIT_ADMIN_USERNAME", ""),
		InitAdminPassword:            envDefault("INIT_ADMIN_PASSWORD", ""),
		InitAdminDisplayName:         envDefault("INIT_ADMIN_DISPLAY_NAME", ""),
		TmpMediaDir:                  envDefault("TMP_MEDIA_DIR", "tmp-media"),
		PublicBaseURL:                strings.TrimRight(envDefault("PUBLIC_BASE_URL", ""), "/"),
		OnDemandRepairURL:            strings.TrimRight(envDefault("ON_DEMAND_REPAIR_URL", ""), "/"),
		OnDemandRepairUser:           envDefault("ON_DEMAND_REPAIR_USERNAME", ""),
		OnDemandRepairPass:           envDefault("ON_DEMAND_REPAIR_PASSWORD", ""),
		OnDemandRepairTimeoutSeconds: envDefaultInt("ON_DEMAND_REPAIR_TIMEOUT_SECONDS", 420),
	}
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
