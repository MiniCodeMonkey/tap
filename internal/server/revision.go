package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// ComputeRevision returns a short content hash identifying one build of a
// deck: the presentation JSON plus the sorted set of component bundle file
// names. tap dev calls this whenever it loads or reloads a deck and passes
// the result to the hub (see WebSocketHub.SetPresentationMeta), so a client
// that reconnects after the deck changed can tell its page is stale (see
// the revision field on the hub's "connected" message, and the client's
// handling of it in frontend/src/lib/stores/websocket.ts).
func ComputeRevision(pres *transformer.TransformedPresentation, componentBundleFiles map[string]ComponentBundleFile) string {
	h := sha256.New()

	if pres != nil {
		if data, err := json.Marshal(pres); err == nil {
			h.Write(data)
		}
	}

	names := make([]string, 0, len(componentBundleFiles))
	for name := range componentBundleFiles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		h.Write([]byte(name))
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:6])
}
