package ssw

import (
	"github.com/caarlos0/env"
	"time"
)

// Config contains configuration for service
type Config struct {
	Timeout      int           `env:"SERVICE_TIMEOUT_MS" envDefault:"500"`
	DelayStart   time.Duration `env:"SERVICE_DELAY_START" envDefault:"0"`
	TickInterval time.Duration `env:"SERVICE_TICK_INTERVAL" envDefault:"1s"`
}

// NewConfig creates a new config instance from environment
func NewConfig() Config {
	cfg := Config{}
	env.Parse(&cfg)
	return cfg
}
