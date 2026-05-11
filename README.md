# AirPipe

Self-hosted file transfer with a passphrase that works anywhere. Files go peer-to-peer between any two devices. The relay never sees your bytes.

![demo](demo.gif)

**Try it:** [pipe.nurdaheim.net](https://pipe.nurdaheim.net)

## How it works in 30 seconds

1. Sender picks a file. Gets a passphrase like `RIVER FALCON MARBLE 42`.
2. Receiver types it at the homepage, runs `airpipe download <PHRASE>`, or scans the QR.
3. Both pair through the relay, then the file streams directly between them over WebRTC.
4. If the receiver isn't online yet, the sender can pick "mailbox" mode instead. The relay holds the encrypted file for 10 minutes.

Same passphrase works for both modes. Receiver doesn't need to know which one the sender chose.

## Self-host

```bash
git clone https://github.com/whaeuser/Airpipe
cd Airpipe
docker compose build && docker compose up -d
```

One Go binary, ~15 MB image. Bundles the landing page, browser sender/receiver, and the install script.

Env vars to tune things (set in `docker-compose.yml` or `.env`):

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Listen port inside container |
| `AIRPIPE_ALLOWED_ORIGINS` | localhost | Comma-separated CORS allowlist, or `*` |
| `AIRPIPE_RATE_LIMIT_PER_MIN` | `60` | Rate limit per IP |
| `AIRPIPE_LOG_FORMAT` | `json` | `json` or `text` |
| `AIRPIPE_MAX_UPLOAD_MB` | `500` | Mailbox upload size limit |
| `PUSHOVER_TOKEN` / `PUSHOVER_USER` | — | Optional Pushover notifications on every send |

## CLI

Install via curl (auto-detects OS and architecture):
```bash
curl -sSL https://github.com/whaeuser/Airpipe/releases/latest/download/airpipe-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') -o /tmp/airpipe && chmod +x /tmp/airpipe && sudo mv /tmp/airpipe /usr/local/bin/airpipe
```

Self-update later: `airpipe update`. Linux + macOS, amd64 + arm64.

### Send

```bash
airpipe report.pdf
```
Or explicitly: `airpipe send report.pdf`. You get a prompt: direct (P2P) or mailbox (relay holds it 10 min). The CLI shows a passphrase, a QR, and a link.

Multiple files or a folder get auto-zipped:
```bash
airpipe file1.txt photos/
```

### Download

```bash
airpipe download RIVER FALCON MARBLE 42
```

### Wait for someone to send to you

```bash
airpipe receive ./downloads
```
Prints a QR. Phone scans it, drops a file, the file lands in `./downloads`. Direct WebRTC, fallback to relay if NAT punching fails.

### Version / update

```bash
airpipe version   # show installed version
airpipe update    # self-update to latest release
```

## Browser to browser, no install

Open `pipe.nurdaheim.net/live`. Get a passphrase + QR. Receiver types the passphrase at the homepage in their browser. Both pair, sender drops a file. No CLI, no app, no account.

## Encryption

NaCl secretbox (Poly1305 + XSalsa20, 256-bit key) on top of DTLS for the live path. The key never leaves the side that generated it. The relay only sees a 16-char room token and ciphertext.

The passphrase derives both the relay token and the encryption key via SHA-256 with domain separation. Same algorithm on CLI and browser.

## Stack

Go relay (gorilla/websocket, pion/webrtc), embedded HTML/CSS/JS frontend (tweetnacl.js for browser crypto), Docker. Single static binary.

## License

MIT
