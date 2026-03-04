# emailverify

A production-ready CLI tool for verifying email addresses using the SMTP `RCPT TO` method — without sending any actual emails.

## How It Works

emailverify probes mail servers directly to check whether an email address exists. For each address it:

1. **Validates syntax** (RFC 5322)
2. **Looks up MX records** for the domain (with automatic fallback to secondary MX servers)
3. **Classifies** the address (disposable provider, role account, free provider)
4. **Opens an SMTP connection** to the mail server and issues `RCPT TO` — the server's response tells you whether the mailbox exists
5. **Optionally checks for catch-all** domains by probing a random address

No emails are ever sent. The connection is closed after the probe.

---

## Features

- SMTP verification via `RCPT TO` (no emails sent)
- STARTTLS support (automatic upgrade when server supports it)
- Syntax validation (RFC 5322 compliant)
- MX record lookup with **automatic fallback** to secondary/tertiary MX servers
- Disposable email detection (500+ domains)
- Role account detection (`admin@`, `support@`, `noreply@`, etc.)
- Free provider detection (`gmail.com`, `yahoo.com`, etc.)
- Catch-all domain detection
- Concurrent bulk verification with configurable worker pool
- **DNS result caching** (10-minute TTL) — dramatically faster for bulk lists with repeated domains
- **Automatic deduplication** of input lists
- Health checks using a known-valid email to detect blocking
- Multiple output formats: JSON, CSV, JSONL, TXT
- Debug mode with full SMTP conversation logging
- Graceful shutdown on Ctrl+C (in-progress results are saved)
- Cross-platform: Linux, macOS, Windows

---

## Installation

### Build from source (recommended)

```bash
git clone https://github.com/nephila016/emailchecker.git
cd emailchecker
go mod tidy
make build
# binary: ./bin/emailverify
```

### go install

```bash
go install github.com/nephila016/emailchecker@latest
```

### Download binary

Pre-built binaries are available on the [Releases](https://github.com/nephila016/emailchecker/releases) page.

---

## Quick Start

```bash
# Verify a single email
emailverify check user@example.com

# Verify from a file, save results to CSV
emailverify bulk -f emails.txt -o results.csv

# Check a domain's mail configuration
emailverify domain example.com --check-catchall --check-spf --check-dmarc
```

---

## Commands

### `check` — Verify a single email

```
emailverify check <email> [flags]
```

```bash
# Basic check (syntax + DNS + SMTP)
emailverify check user@example.com

# Syntax and DNS only (no SMTP connection)
emailverify check user@example.com --skip-smtp

# Use a specific SMTP server instead of auto-resolving MX
emailverify check user@example.com -i mail.example.com -p 25

# Detect catch-all (sends a random probe address after the real one)
emailverify check user@example.com --catch-all

# JSON output (pipe-friendly)
emailverify check user@example.com --json

# Save to file (format auto-detected from extension)
emailverify check user@example.com -o result.json

# Full debug — see every SMTP command and response
emailverify check user@example.com -ddd
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-i, --ip` | _(auto MX)_ | Custom SMTP server hostname or IP |
| `-p, --port` | `25` | SMTP port |
| `-t, --timeout` | `15` | Connection timeout (seconds) |
| `--from` | `test@gmail.com` | `MAIL FROM` address used during probe |
| `--helo` | `mail.verification-check.com` | `EHLO` domain sent to server |
| `--skip-smtp` | `false` | Skip SMTP — syntax and DNS only |
| `--catch-all` | `false` | Test whether domain accepts all mail |
| `--json` | `false` | Print result as JSON to stdout |
| `-o, --output` | | Save result to file |

---

### `bulk` — Verify many emails from a file

```
emailverify bulk -f <file> [flags]
```

Input file format: one email per line. Blank lines and lines starting with `#` are ignored. **Duplicate addresses are removed automatically** before processing.

```bash
# Basic bulk run
emailverify bulk -f emails.txt -o results.csv

# 5 workers, 3s delay, save as JSON Lines
emailverify bulk -f emails.txt -w 5 -d 3 -o results.jsonl

# Use a custom SMTP server
emailverify bulk -f emails.txt -i mail.example.com -p 25

# Health checks every 10 emails to detect if the server is blocking you
emailverify bulk -f emails.txt --health-email info@yourdomain.com --health-interval 10

# Syntax + DNS only (no SMTP, very fast)
emailverify bulk -f emails.txt --skip-smtp -o results.csv

# Full example
emailverify bulk \
  -f emails.txt \
  -w 3 \
  -d 2 \
  --jitter 1 \
  --timeout 15 \
  --health-email info@yourdomain.com \
  --health-interval 10 \
  --catch-all \
  -o results.csv
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-f, --file` | _(required)_ | Input file path |
| `-i, --ip` | _(auto MX)_ | Custom SMTP server hostname or IP |
| `-p, --port` | `25` | SMTP port |
| `-o, --output` | `results.csv` | Output file (format from extension) |
| `-w, --workers` | `3` | Number of concurrent workers |
| `-d, --delay` | `2.0` | Seconds between verifications per worker |
| `--jitter` | `1.0` | Max random extra delay added to `--delay` |
| `-t, --timeout` | `15` | SMTP connection timeout (seconds) |
| `--from` | `test@gmail.com` | `MAIL FROM` address |
| `--helo` | `mail.verification-check.com` | `EHLO` domain |
| `--health-email` | | Known-valid address for periodic health checks |
| `--health-interval` | `10` | Run health check every N emails |
| `--skip-smtp` | `false` | Skip SMTP — syntax and DNS only |
| `--catch-all` | `false` | Test each domain for catch-all |
| `--proxy` | | _(not yet implemented)_ |
| `--resume` | | _(not yet implemented)_ |

---

### `domain` — Check a domain's mail configuration

```
emailverify domain <domain> [flags]
```

```bash
# MX records + classification
emailverify domain example.com

# Full check: MX + SPF + DMARC + catch-all
emailverify domain example.com --check-spf --check-dmarc --check-catchall

# JSON output
emailverify domain example.com --check-spf --check-dmarc --json
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--check-catchall` | `false` | Send a random probe to test catch-all |
| `--check-spf` | `false` | Look up SPF TXT record |
| `--check-dmarc` | `false` | Look up DMARC TXT record |
| `--json` | `false` | Output as JSON |
| `-t, --timeout` | `15` | DNS/SMTP timeout (seconds) |

---

## Global Flags

These flags work on all commands:

| Flag | Description |
|------|-------------|
| `-d` / `-dd` / `-ddd` | Debug verbosity: basic / detailed / full SMTP trace |
| `--debug-file <path>` | Write debug output to a file instead of stderr |
| `-q, --quiet` | Suppress all output except results |
| `--no-color` | Disable colored output |
| `-c, --config <path>` | Config file path (default: `~/.emailverify.yaml`) |

---

## Output Formats

Format is automatically detected from the output file extension:

| Extension | Format |
|-----------|--------|
| `.csv` | CSV with headers |
| `.json` | JSON array |
| `.jsonl` | JSON Lines (one object per line, good for streaming/large files) |
| `.txt` | Plain text — valid emails only, one per line |

---

## Result Statuses

| Status | Meaning |
|--------|---------|
| `valid` | Mailbox accepted by the server (SMTP `250`/`251`) |
| `invalid` | Mailbox rejected by the server (SMTP `550`–`559`) |
| `unknown` | Server could not confirm either way (greylisted, `252`, temp error) |
| `risky` | Domain is catch-all — the address format is valid but delivery is uncertain |
| `error` | Connection or protocol error during verification |

### SMTP Response Codes

| Code | Meaning |
|------|---------|
| `250` | Recipient accepted — mailbox exists |
| `251` | Not local, will forward — treated as valid |
| `252` | Cannot verify, will attempt delivery — treated as unknown |
| `450`–`459` | Temporary failure (greylisting, busy) — treated as unknown |
| `530` | STARTTLS required — tool upgrades automatically and retries |
| `550`–`559` | Permanent rejection — mailbox does not exist |

---

## Configuration File

Create `~/.emailverify.yaml` (copy from `.emailverify.yaml.example` in the repo):

```yaml
defaults:
  timeout: 15
  delay: 2.0
  jitter: 1.0
  workers: 3
  port: 25
  from_address: test@gmail.com
  helo_domain: mail.verification-check.com

health:
  email: ""
  interval: 10

output:
  format: csv
  colored: true

debug:
  enabled: false
  level: 1
```

---

## Debug Mode

Debug mode logs every SMTP command and response so you can see exactly what's happening:

```bash
# Level 1: info messages
emailverify check user@example.com -d

# Level 2: detailed (DNS results, SMTP banner, TLS info)
emailverify check user@example.com -dd

# Level 3: full trace (every SMTP send/recv line)
emailverify check user@example.com -ddd

# Write debug output to a file (useful for bulk runs)
emailverify bulk -f emails.txt -dd --debug-file smtp-debug.log
```

---

## Tips for Bulk Verification

**Avoid being blocked:**
- Start with `--workers 2` and `--delay 3` — increase only if it's working cleanly
- Use `--jitter 1` to add randomness and avoid rate-limit patterns
- Always set `--health-email` to a known-valid address you control — if it starts failing, the server is likely blocking you
- Different servers have different tolerances; enterprise servers (Google, Microsoft) are strict

**Speed up large lists:**
- DNS results are cached for 10 minutes per domain — lists with many emails at the same company (e.g. thousands of `@google.com`) are fast after the first lookup
- Use `--skip-smtp` for a quick first pass to filter out bad syntax and dead domains before running the full SMTP check
- JSONL output (`.jsonl`) is more memory-efficient than JSON for very large result sets

**Input file format:**
```
# This is a comment, will be skipped
user@example.com
another@example.com

# Blank lines are also skipped
duplicate@example.com
duplicate@example.com   # this duplicate will be removed automatically
```

---

## Building

```bash
# Build binary to ./bin/emailverify
make build

# Build for all platforms
make build-all

# Run tests
make test

# Show all targets
make help
```

---

## License

MIT License
