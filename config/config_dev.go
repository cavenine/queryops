//go:build dev

package config

import "os"

func Load() *Config {
	cfg := loadBase()
	cfg.Environment = Dev

	if _, ok := os.LookupEnv("PUBSUB_AUTO_INIT_SCHEMA"); !ok {
		cfg.PubSubAutoInitSchema = true
	}

	return cfg
}
