package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// revisionSalt is generated once when the process starts and mixed into
// every revision hash below, so the revision a client sees on connect is
// never a pure function of the deck's content alone: the JSON ComputeRevision
// hashes is built from the full TransformedPresentation, including a
// driver's connection settings, and those can carry a literal secret (see
// transformer.PublicConfig, which is what actually reaches a client - the
// revision is computed from more than that). Salting the hash means a
// twelve-character revision can never be tested offline against a guessed
// secret: reproducing it needs this process's salt too, and that never
// leaves the process. The salt stays fixed for the process's lifetime, so
// the hash stays deterministic within one run, which is what change
// detection needs.
var revisionSalt = randomRevisionSalt()

// randomRevisionSalt returns 16 random bytes, or 16 zero bytes if the
// system's random source is unavailable: ComputeRevision must stay total,
// and losing the salt's secrecy in that unlikely case still leaves change
// detection working.
func randomRevisionSalt() []byte {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	return salt
}

// ComputeRevision returns a short content hash identifying one build of a
// deck: the presentation JSON plus the sorted set of component bundle file
// names, salted with this process's revisionSalt. tap dev calls this
// whenever it loads or reloads a deck and passes the result to the hub (see
// WebSocketHub.SetPresentationMeta), so a client that reconnects after the
// deck changed can tell its page is stale (see the revision field on the
// hub's "connected" message, and the client's handling of it in
// frontend/src/lib/stores/websocket.ts).
func ComputeRevision(pres *transformer.TransformedPresentation, componentBundleFiles map[string]ComponentBundleFile) string {
	h := sha256.New()
	h.Write(revisionSalt)

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
