package sqlite

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Config struct {
	Path string
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Path) == "" {
		return fmt.Errorf("sqlite path is required")
	}
	return nil
}

func (c Config) NormalizedPath() string {
	trimmed := strings.TrimSpace(c.Path)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}
