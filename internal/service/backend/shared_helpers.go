package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func indexEntryVersion(parts ...string) string {
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(strings.TrimSpace(part))
		b.WriteString("\n")
	}
	digest := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(digest[:8])
}
