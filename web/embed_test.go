package web

import (
	"io/fs"
	"testing"
)

func TestEmbeddedStaticAssets_DoNotShipMemoriesPage(t *testing.T) {
	if _, err := fs.Stat(Static, "memories.html"); err == nil {
		t.Fatal("web.Static still embeds memories.html")
	}
}
