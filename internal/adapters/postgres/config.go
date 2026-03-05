package postgres

import (
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultMaxConns int32 = 8
	defaultMinConns int32 = 0
)

const (
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
)

type Config struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.DSN) == "" {
		return fmt.Errorf("postgres dsn is required")
	}
	if c.MaxConns < 0 {
		return fmt.Errorf("max_conns must be >= 0")
	}
	if c.MinConns < 0 {
		return fmt.Errorf("min_conns must be >= 0")
	}
	maxConns := c.MaxConns
	if maxConns == 0 {
		maxConns = defaultMaxConns
	}
	minConns := c.MinConns
	if minConns > maxConns {
		return fmt.Errorf("min_conns must be <= max_conns")
	}
	if c.ConnMaxLifetime < 0 {
		return fmt.Errorf("conn_max_lifetime must be >= 0")
	}
	if c.ConnMaxIdleTime < 0 {
		return fmt.Errorf("conn_max_idle_time must be >= 0")
	}
	return nil
}

func (c Config) PoolConfig() (*pgxpool.Config, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	cfg, err := pgxpool.ParseConfig(strings.TrimSpace(c.DSN))
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	if c.MaxConns == 0 {
		cfg.MaxConns = defaultMaxConns
	} else {
		cfg.MaxConns = c.MaxConns
	}
	cfg.MinConns = c.MinConns

	if c.ConnMaxLifetime == 0 {
		cfg.MaxConnLifetime = defaultConnMaxLifetime
	} else {
		cfg.MaxConnLifetime = c.ConnMaxLifetime
	}
	if c.ConnMaxIdleTime == 0 {
		cfg.MaxConnIdleTime = defaultConnMaxIdleTime
	} else {
		cfg.MaxConnIdleTime = c.ConnMaxIdleTime
	}

	return cfg, nil
}
