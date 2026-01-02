# emailchecker

Production-ready CLI tool for email verification using SMTP RCPT TO method. Verifies email addresses without sending actual emails.

## Features

- SMTP verification with STARTTLS support
- Syntax validation (RFC 5322)
- MX record lookup
- Disposable email detection (500+ domains)
- Role account detection (admin@, support@, etc.)
- Free provider detection
- Catch-all domain detection
- Concurrent bulk verification
- Health checks with known-valid email
- Multiple output formats (JSON, CSV, JSONL, TXT)
- Debug mode with detailed SMTP conversation logging
- Cross-platform (Linux, macOS, Windows)

## Installation

### From Source

```bash
go install github.com/nephila016/emailchecker@latest
```

### Build from Source

```bash
git clone https://github.com/nephila016/emailchecker.git
cd emailchecker
go mod tidy
make build
```

### Download Binary

Download pre-built binaries from the [Releases](https://github.com/nephila016/emailchecker/releases) page.

## Usage

### Verify Single Email

```bash
# Basic verification
emailchecker check user@example.com

# With custom server
emailchecker check user@example.com -i mail.example.com -p 25

# Skip SMTP (syntax/DNS only)
emailchecker check user@example.com --skip-smtp

# JSON output
emailchecker check user@example.com --json

# With debug output
emailchecker check user@example.com -d
```

### Bulk Verification

```bash
# Basic bulk verification
emailchecker bulk -f emails.txt -o results.csv

# With custom settings
emailchecker bulk -f emails.txt -i mail.example.com -p 25 -w 5 -d 3

# With health checks
emailchecker bulk -f emails.txt --health-email info@example.com --health-interval 10

# Full options
emailchecker bulk -f emails.txt \
  -i mail.example.com \
  -p 25 \
  -w 3 \
  -d 2 \
  --jitter 1 \
  --health-email info@example.com \
  -o results.json
```

### Domain Check

```bash
# Basic domain check
emailchecker domain example.com

# With catch-all detection
emailchecker domain example.com --check-catchall

# With SPF and DMARC
emailchecker domain example.com --check-spf --check-dmarc

# JSON output
emailchecker domain example.com --json
```

## Commands

### check

Verify a single email address.

```
emailchecker check <email> [flags]

Flags:
  -i, --ip string       Custom SMTP server IP/hostname
  -p, --port int        SMTP port (default 25)
  -t, --timeout int     Connection timeout in seconds (default 15)
      --from string     MAIL FROM address (default "test@gmail.com")
      --helo string     EHLO domain (default "mail.verification-check.com")
      --skip-smtp       Skip SMTP verification
  -o, --output string   Output file
      --json            Output as JSON to stdout
      --catch-all       Check for catch-all domain
```

### bulk

Verify multiple emails from a file.

```
emailchecker bulk [flags]

Flags:
  -f, --file string           Input file with emails (required)
  -i, --ip string             Custom SMTP server IP/hostname
  -p, --port int              SMTP port (default 25)
  -o, --output string         Output file (default "results.csv")
  -w, --workers int           Number of concurrent workers (default 3)
  -d, --delay float           Delay between checks in seconds (default 2)
      --jitter float          Random jitter added to delay (default 1)
  -t, --timeout int           Connection timeout in seconds (default 15)
      --from string           MAIL FROM address (default "test@gmail.com")
      --helo string           EHLO domain
      --health-email string   Known-valid email for health checks
      --health-interval int   Health check every N emails (default 10)
      --skip-smtp             Skip SMTP verification
      --catch-all             Check for catch-all domains
```

### domain

Check domain-level information.

```
emailchecker domain <domain> [flags]

Flags:
      --check-catchall   Check for catch-all configuration
      --check-spf        Check SPF record
      --check-dmarc      Check DMARC record
      --json             Output as JSON
  -t, --timeout int      Timeout in seconds (default 15)
```

## Global Flags

```
  -c, --config string      Config file (default $HOME/.emailchecker.yaml)
  -d, --debug              Enable debug mode (use -d, -dd, -ddd for more detail)
      --debug-file string  Write debug output to file
  -q, --quiet              Quiet mode - minimal output
      --no-color           Disable colored output
```

## Debug Mode

Use debug flags to see detailed SMTP conversation:

```bash
# Basic debug
emailchecker check user@example.com -d

# Detailed SMTP conversation
emailchecker check user@example.com -dd

# Full packet dumps
emailchecker check user@example.com -ddd

# Write debug to file
emailchecker check user@example.com -d --debug-file debug.log
```

## Output Formats

The tool automatically detects output format from file extension:

| Extension | Format |
|-----------|--------|
| .json     | JSON array |
| .jsonl    | JSON Lines (one per line) |
| .csv      | CSV with headers |
| .txt      | Plain text (valid emails only) |

## Configuration File

Create `~/.emailchecker.yaml`:

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

## Result Status

| Status | Description |
|--------|-------------|
| valid | Email exists and is deliverable |
| invalid | Email does not exist |
| unknown | Server cannot confirm (greylist, etc.) |
| risky | Catch-all domain or other uncertainty |
| error | Verification error occurred |

## SMTP Response Codes

| Code | Meaning |
|------|---------|
| 250 | Valid - recipient accepted |
| 251 | Valid - user not local, will forward |
| 252 | Unknown - cannot verify, will try delivery |
| 550 | Invalid - user does not exist |
| 551 | Invalid - user not local |
| 553 | Invalid - mailbox name not allowed |
| 450 | Temp error - mailbox busy |
| 554 | Error - transaction failed |

## Best Practices

1. **Use Health Checks**: Always use `--health-email` with a known-valid address to detect blocking
2. **Rate Limiting**: Use appropriate delays to avoid being blocked (2-5 seconds recommended)
3. **Workers**: Start with 3 workers, increase only if server allows
4. **Jitter**: Add random jitter to appear more human-like
5. **Monitor**: Watch for increased errors which may indicate blocking

## License

MIT License
