# Repository Review: accuracy, security, and speed

Date: 2026-03-05

## Method
- Ran static read-through of CLI, verifier, DNS, SMTP, and worker-pool paths.
- Ran `go test ./...` to verify compile-level health.

## Findings

### 1) Accuracy: `domain` command does SPF/DMARC lookups even when flags are not set
**Severity:** Medium

`runDomain()` calls `v.CheckDomain(domain)` first; `CheckDomain()` *always* performs SPF and DMARC lookups. Then `runDomain()` conditionally performs SPF/DMARC lookups again when flags are set.

Impact:
- Behavior does not match CLI contract that `--check-spf` and `--check-dmarc` are optional checks.
- Extra DNS traffic and latency happen even when users did not request those checks.

Recommendation:
- Move SPF/DMARC lookups entirely to `runDomain()` behind flags, or add options to `CheckDomain()` so lookups are explicit.

---

### 2) Accuracy: catch-all check in `domain` ignores configured SMTP port/from/helo
**Severity:** Medium

`performCatchAllCheck()` hardcodes SMTP port `25`, `FromAddress`, and `HELODomain` values instead of using command configuration.

Impact:
- Results can be wrong in environments where submission/verification requires non-default port or custom identity.
- Inconsistent behavior versus `check`/`bulk` commands.

Recommendation:
- Build `SMTPConfig` from user-provided command settings (`--port`, `--from`, `--helo`) and propagate to catch-all checks.

---

### 3) Security: default verifier config disables TLS certificate verification
**Severity:** Medium

`DefaultConfig()` sets `SkipTLSVerify: true`.

Impact:
- If consumers instantiate verifier with defaults (especially as a library), STARTTLS certificate validation is disabled.
- Increases MITM risk and allows spoofed SMTP endpoints to be treated as trusted TLS sessions.

Recommendation:
- Change default to `false` (verify certificates).
- Expose a clearly named opt-out flag for insecure environments.

---

### 4) Security/Privacy: SMTP debug logs can leak full target emails
**Severity:** Low

`SMTPSend()` logs full SMTP commands including `RCPT TO:<email>` at detailed debug levels.

Impact:
- Debug files can contain PII email addresses.

Recommendation:
- Add an option to redact mailbox local parts in logs (e.g., `u***@example.com`).
- Keep full logging only behind explicit "unsafe debug" mode.

---

### 5) Speed: per-result `writer.Flush()` in bulk mode reduces throughput
**Severity:** Medium

Bulk callback flushes output after every single result.

Impact:
- Frequent fsync-like behavior can bottleneck large runs.
- Particularly expensive on networked filesystems and spinning disks.

Recommendation:
- Flush in batches (e.g., every N results or every M seconds) and on graceful shutdown.

---

### 6) Speed/Maintainability: `--reconnect` flag is accepted but unused
**Severity:** Low

Bulk command exposes `--reconnect`, but there is no implementation path using this value.

Impact:
- User confusion and false expectations.
- Additional support overhead.

Recommendation:
- Implement reconnect logic or remove/deprecate the flag until implemented.

## Positive notes
- SMTP response size is capped to prevent unbounded response growth.
- DNS MX cache exists with TTL and reduces repeated lookups for bulk workloads.
- Worker pool has cancellation paths and graceful stop behavior.

## Validation run
- `go test ./...` passed (no test files, compile checks succeeded).
