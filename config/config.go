package config

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Environment string

const (
	Dev  Environment = "dev"
	Prod Environment = "prod"
)

type Config struct {
	Environment Environment `mapstructure:"-"`

	Host          string     `mapstructure:"HOST"`
	Port          string     `mapstructure:"PORT"`
	LogLevel      slog.Level `mapstructure:"-"`
	SessionSecret string     `mapstructure:"SESSION_SECRET"`

	DatabaseURL       string `mapstructure:"DATABASE_URL"`
	DatabaseMinConns  int32  `mapstructure:"DATABASE_MIN_CONNS"`
	DatabaseMaxConns  int32  `mapstructure:"DATABASE_MAX_CONNS"`
	DatabaseMaxIdle   int32  `mapstructure:"DATABASE_MAX_IDLE"`
	DatabaseMaxLifeMs int64  `mapstructure:"DATABASE_MAX_LIFE_MS"`

	AutoMigrate          bool `mapstructure:"AUTO_MIGRATE"`
	BackgroundProcessing bool `mapstructure:"BACKGROUND_PROCESSING"`

	// WebAuthn configuration for passkey authentication
	WebAuthnRPID          string `mapstructure:"WEBAUTHN_RP_ID"`           // Domain name (e.g., "localhost" or "example.com")
	WebAuthnRPOrigin      string `mapstructure:"WEBAUTHN_RP_ORIGIN"`       // Full origin URL (e.g., "http://localhost:8080")
	WebAuthnRPDisplayName string `mapstructure:"WEBAUTHN_RP_DISPLAY_NAME"` // Human-readable site name
}

var (
	Global *Config
	once   sync.Once
)

func init() {
	once.Do(func() {
		Global = Load()
	})
}

func loadBase() *Config {
	_ = godotenv.Load()

	v := viper.New()
	v.AutomaticEnv()

	v.SetDefault("HOST", "0.0.0.0")
	v.SetDefault("PORT", "8080")
	v.SetDefault("LOG_LEVEL", "INFO")
	v.SetDefault("SESSION_SECRET", "session-secret")
	v.SetDefault("DATABASE_URL", "postgres://queryops:queryops@localhost:5432/queryops?sslmode=disable")
	v.SetDefault("AUTO_MIGRATE", true)
	v.SetDefault("BACKGROUND_PROCESSING", true)
	v.SetDefault("WEBAUTHN_RP_ID", "localhost")
	v.SetDefault("WEBAUTHN_RP_ORIGIN", "http://localhost:8080")
	v.SetDefault("WEBAUTHN_RP_DISPLAY_NAME", "QueryOps")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		// Fallback to sensible defaults if unmarshal fails.
		cfg = Config{
			Host:          "0.0.0.0",
			Port:          "8080",
			SessionSecret: "session-secret",
		}
	}

	level := strings.ToUpper(v.GetString("LOG_LEVEL"))
	switch level {
	case "DEBUG":
		cfg.LogLevel = slog.LevelDebug
	case "INFO":
		cfg.LogLevel = slog.LevelInfo
	case "WARN", "WARNING":
		cfg.LogLevel = slog.LevelWarn
	case "ERROR":
		cfg.LogLevel = slog.LevelError
	default:
		cfg.LogLevel = slog.LevelInfo
	}

	return &cfg
}
