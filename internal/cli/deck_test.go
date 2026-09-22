package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
)

// TestPrepareDeck_CarriesRevision checks that the temporary server
// prepareDeck starts carries a non-empty revision on /api/presentation.
// tap export pdf and tap export images render their print pages through
// this server, which has no WebSocket, so /api/presentation's revision is
// the only way the page's ready signal can report one; without this, that
// signal would stay empty for every export, not just tap dev.
func TestPrepareDeck_CarriesRevision(t *testing.T) {
	deckPath, err := filepath.Abs(filepath.Join("..", "..", "examples", "basic.md"))
	if err != nil {
		t.Fatalf("failed to resolve deck path: %v", err)
	}
	baseDir := filepath.Dir(deckPath)

	cfg, err := config.Load(deckPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	srv, _, _, prepareErrs, _, err := prepareDeck(deckPath, cfg, baseDir)
	if err != nil {
		t.Fatalf("prepareDeck() error = %v", err)
	}
	if len(prepareErrs) > 0 {
		t.Fatalf("unexpected component build errors from prepareDeck: %v", prepareErrs)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	resp, err := http.Get("http://localhost:" + strconv.Itoa(srv.Port()) + "/api/presentation")
	if err != nil {
		t.Fatalf("GET /api/presentation error = %v", err)
	}
	defer resp.Body.Close()

	var body struct {
		Revision string `json:"revision"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding the body: %v", err)
	}
	if body.Revision == "" {
		t.Error("revision = \"\", want a non-empty revision")
	}
}
