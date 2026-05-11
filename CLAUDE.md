# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go test ./...          # run all tests
go test ./internal/transfer/...  # run a single package's tests
go vet ./...           # lint
go build ./cmd/airpipe # build the CLI
go build ./cmd/relay   # build the relay server
docker compose up -d   # run relay locally on :8088
```

## Architecture

Two binaries share one module (`github.com/sanyamgarg/airpipe`):

**`cmd/relay`** — the server. Single Go HTTP process that:
- Brokers WebSocket signaling rooms (`/ws/{token}`) used for WebRTC negotiation and fallback WS streaming
- Stores mailbox uploads in-memory + temp files (`FileStore`), auto-expired after 10 minutes
- Serves the embedded static frontend (HTML/CSS/JS via `//go:embed static/*`)
- Config via env vars: `PORT`, `AIRPIPE_ALLOWED_ORIGINS`, `AIRPIPE_RATE_LIMIT_PER_MIN`, `AIRPIPE_LOG_FORMAT`

**`cmd/airpipe`** — the CLI. Two transfer modes:
- **P2P (direct):** sender and receiver WebRTC-negotiate through the relay, then stream directly. Relay only forwards encrypted signaling bytes.
- **Mailbox:** sender encrypts and uploads the file to the relay; receiver fetches and decrypts it later. File never exists unencrypted on the relay.

### Transfer flow

1. Sender generates a passphrase (`WORD WORD WORD NN`).
2. `passphrase.DeriveToken` → 16-char hex room token (SHA-256, domain-prefixed).
3. `passphrase.DeriveKey` → 32-byte NaCl key (SHA-256, domain-prefixed). Key is separate from the token.
4. **P2P path:** sender opens WS room, receiver joins, both negotiate via SDP/ICE messages wrapped in the encrypted signaling protocol (`internal/transfer/signaling.go`). Falls back to relay-proxied WS if WebRTC fails.
5. **Mailbox path:** sender encrypts (`crypto.Encrypt` = NaCl secretbox) and POSTs to `/upload` with the derived token; receiver GETs `/raw/{token}` and decrypts locally.

### Internal packages

| Package | Purpose |
|---|---|
| `internal/crypto` | NaCl secretbox (XSalsa20+Poly1305). `Encrypt`/`Decrypt` for whole blobs; `EncryptChunk`/`DecryptChunk` alias for streaming. |
| `internal/transfer` | Binary framing protocol (`protocol.go`), `Sender`/`Receiver` types, WebRTC signaling negotiation. All wire bytes are encrypted. |
| `internal/passphrase` | Passphrase generation (1024-word list + 2-digit suffix) and deterministic derivation of token + key. |
| `internal/p2p` | Thin wrapper around `pion/webrtc` for the WebRTC data channel. |
| `internal/archive` | Zip multiple files/dirs into a single temp file before sending. |
| `internal/progress` | Progress bar helpers. |
| `internal/qr` | Terminal QR code rendering. |

### Wire protocol

Messages are: `[1-byte type][4-byte big-endian payload length][payload]`, then NaCl-encrypted before hitting the wire. Protocol version is checked on connect (`MsgTypeVersion = 0x20`, current version = 2).

### Browser frontend

The relay embeds HTML/JS pages for browser-based transfers. Browser crypto uses `tweetnacl.js` (the same NaCl primitives as the Go code). Passphrase derivation is reimplemented in JS using the same domain-separated SHA-256 scheme so CLI and browser interoperate.

### Release

- Every push to `main` builds and pushes `ghcr.io/sanyam-g/airpipe-relay:latest`.
- Tagged pushes (`v*`) additionally cross-compile CLI binaries for linux/darwin × amd64/arm64 and attach them to the GitHub release.
