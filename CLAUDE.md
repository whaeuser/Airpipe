# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go test ./...          # run all tests
go test ./internal/transfer/...  # run a single package's tests
go vet ./...           # lint
go build ./cmd/drop # build the CLI
go build ./cmd/relay   # build the relay server
docker compose build && docker compose up -d  # rebuild and restart relay on :8088
```

## Deployment (whaeuser fork)

This is a fork of the original project, customised for self-hosting on `drop.volt-logik.io` (alias: `pipe.nurdaheim.net`).

- **Relay:** runs in Docker on port 8088, reverse-proxied via the host
- **CLI binary:** `/usr/local/bin/drop` → symlink to `/root/Airpipe/drop-linux-amd64`
- **Config:** sensitive values (Pushover credentials) live in `.env` (gitignored); all other relay config is in `docker-compose.yml`
- **Releases:** tag `vX.Y.Z` → GitHub Actions builds CLI binaries for linux/darwin × amd64/arm64 and attaches them to the release. The relay Docker image is built locally, not pushed to a registry.

To release a new version:
```bash
git tag vX.Y.Z && git push origin vX.Y.Z
# after Actions complete:
drop update   # on each machine
```

## Relay environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Listen port inside container |
| `DROP_ALLOWED_ORIGINS` | (drop.volt-logik.io + pipe.nurdaheim.net + localhost) | Comma-separated CORS allowlist, or `*` |
| `DROP_RATE_LIMIT_PER_MIN` | `60` | WebSocket + upload rate limit per IP |
| `DROP_LOG_FORMAT` | `json` | `json` or `text` |
| `DROP_MAX_UPLOAD_MB` | `500` | Mailbox upload size limit in MB |
| `PUSHOVER_TOKEN` | — | Pushover app token (put in `.env`) |
| `PUSHOVER_USER` | — | Pushover user key (put in `.env`) |

Pushover notifications fire on every mailbox upload and every new P2P room (first client joins). If either var is unset, notifications are silently skipped.

## Architecture

Two binaries share one module (`github.com/whaeuser/drop`):

**`cmd/relay`** — the server. Single Go HTTP process that:
- Brokers WebSocket signaling rooms (`/ws/{token}`) used for WebRTC negotiation and fallback WS streaming
- Stores mailbox uploads in-memory + temp files (`FileStore`), auto-expired after 10 minutes
- Serves the embedded static frontend (HTML/CSS/JS via `//go:embed static/*`)

**`cmd/drop`** — the CLI. Two transfer modes:
- **P2P (direct):** sender and receiver WebRTC-negotiate through the relay, then stream directly. Relay only forwards encrypted signaling bytes.
- **Mailbox:** sender encrypts and uploads the file to the relay; receiver fetches and decrypts it later. File never exists unencrypted on the relay.

Bare file arguments are treated as implicit `send`: `drop file.txt` = `drop send file.txt`.

### Transfer flow

1. Sender generates a passphrase (`WORD WORD NN`).
2. `passphrase.DeriveToken` → 16-char hex room token (SHA-256, domain-prefixed).
3. `passphrase.DeriveKey` → 32-byte NaCl key (SHA-256, domain-prefixed). Key is separate from the token.
4. **P2P path:** sender opens WS room, receiver joins, both negotiate via SDP/ICE messages. If WebRTC fails, sender sends `P2PFail` and both fall back to relay-proxied WS streaming. The receiver reuses the existing `startWSReader` channel on fallback to avoid losing the first metadata message.
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

### Release workflow

Tagged pushes (`v*`) cross-compile CLI binaries for linux/darwin × amd64/arm64 and attach them to the GitHub release. No Docker image is pushed to a registry — the relay image is built locally with `docker compose build`.
