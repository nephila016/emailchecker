package verifier

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/nephila016/emailchecker/internal/debug"
)

// maxSMTPResponseSize limits the total bytes read in a single SMTP response.
// Protects against memory exhaustion from malicious/buggy servers.
const maxSMTPResponseSize = 16 * 1024 // 16 KB

// SMTPConfig holds SMTP connection configuration
type SMTPConfig struct {
	Host          string
	Port          int
	Timeout       time.Duration
	FromAddress   string
	HELODomain    string
	ForceTLS      bool
	SkipTLSVerify bool
}

// DefaultSMTPConfig returns default SMTP configuration
func DefaultSMTPConfig() *SMTPConfig {
	return &SMTPConfig{
		Port:          25,
		Timeout:       15 * time.Second,
		FromAddress:   "test@gmail.com",
		HELODomain:    "mail.verification-check.com",
		SkipTLSVerify: false, // Verify TLS certs by default (secure default)
	}
}

// SMTPConnection represents an SMTP connection
type SMTPConnection struct {
	conn     net.Conn
	reader   *bufio.Reader
	config   *SMTPConfig
	useTLS   bool
	banner   string
	features map[string]bool
}

// NewSMTPConnection creates a new SMTP connection
func NewSMTPConnection(config *SMTPConfig) *SMTPConnection {
	return &SMTPConnection{
		config:   config,
		features: make(map[string]bool),
	}
}

// Connect establishes connection to SMTP server
func (s *SMTPConnection) Connect() error {
	log := debug.GetLogger()
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	timer := log.StartTimer("SMTP", fmt.Sprintf("Connecting to %s", addr))

	dialer := &net.Dialer{Timeout: s.config.Timeout}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		timer.Stop()
		log.Error("SMTP", "Connection failed: %v", err)
		return fmt.Errorf("connection failed: %w", err)
	}

	s.conn = conn
	s.reader = bufio.NewReader(conn)
	conn.SetDeadline(time.Now().Add(s.config.Timeout))

	timer.Stop()
	log.Success("SMTP", "Connected to %s (latency: %v)", addr, timer.Elapsed())

	// Read banner
	banner, err := s.readResponse()
	if err != nil {
		return fmt.Errorf("failed to read banner: %w", err)
	}
	s.banner = banner

	code := s.parseCode(banner)
	if code != 220 {
		return fmt.Errorf("unexpected banner code %d: %s", code, banner)
	}

	log.Detail("SMTP", "Banner: %s", strings.TrimSpace(banner))

	return nil
}

// EHLO sends EHLO command and parses capabilities
func (s *SMTPConnection) EHLO() error {
	log := debug.GetLogger()

	response, err := s.sendCommand(fmt.Sprintf("EHLO %s", s.config.HELODomain))
	if err != nil {
		return err
	}

	code := s.parseCode(response)
	if code != 250 {
		// Try HELO as fallback
		log.Detail("SMTP", "EHLO failed, trying HELO")
		response, err = s.sendCommand(fmt.Sprintf("HELO %s", s.config.HELODomain))
		if err != nil {
			return err
		}
		code = s.parseCode(response)
		if code != 250 {
			return fmt.Errorf("HELO failed with code %d: %s", code, response)
		}
		return nil
	}

	// Parse EHLO features
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 4 {
			continue
		}
		// Remove response code prefix (e.g. "250-STARTTLS" → "STARTTLS")
		feature := strings.ToUpper(strings.TrimSpace(line[4:]))
		if strings.HasPrefix(feature, "250") {
			feature = strings.TrimSpace(feature[3:])
		}
		feature = strings.Split(feature, " ")[0]
		if feature != "" {
			s.features[feature] = true
			log.Trace("SMTP", "Feature: %s", feature)
		}
	}

	return nil
}

// StartTLS upgrades connection to TLS if supported
func (s *SMTPConnection) StartTLS() error {
	log := debug.GetLogger()

	if !s.features["STARTTLS"] {
		log.Detail("SMTP", "STARTTLS not supported by server")
		return nil
	}

	log.Info("SMTP", "Initiating STARTTLS")

	response, err := s.sendCommand("STARTTLS")
	if err != nil {
		return err
	}

	code := s.parseCode(response)
	if code != 220 {
		return fmt.Errorf("STARTTLS failed with code %d: %s", code, response)
	}

	// Upgrade to TLS
	tlsConfig := &tls.Config{
		ServerName:         s.config.Host,
		InsecureSkipVerify: s.config.SkipTLSVerify, //nolint:gosec // controlled by caller
	}

	tlsConn := tls.Client(s.conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		return fmt.Errorf("TLS handshake failed: %w", err)
	}

	s.conn = tlsConn
	s.reader = bufio.NewReader(tlsConn)
	s.useTLS = true

	state := tlsConn.ConnectionState()
	log.Success("SMTP", "TLS established (version: %s, cipher: %s)",
		tlsVersionString(state.Version), tls.CipherSuiteName(state.CipherSuite))

	// Re-send EHLO after STARTTLS (required by RFC)
	s.features = make(map[string]bool)
	return s.EHLO()
}

// MailFrom sends MAIL FROM command
func (s *SMTPConnection) MailFrom(from string) error {
	log := debug.GetLogger()

	response, err := s.sendCommand(fmt.Sprintf("MAIL FROM:<%s>", from))
	if err != nil {
		return err
	}

	code := s.parseCode(response)

	// Check if STARTTLS is required
	if code == 530 && strings.Contains(strings.ToUpper(response), "STARTTLS") {
		log.Detail("SMTP", "Server requires STARTTLS")
		if err := s.StartTLS(); err != nil {
			return err
		}
		// Retry MAIL FROM after TLS upgrade
		response, err = s.sendCommand(fmt.Sprintf("MAIL FROM:<%s>", from))
		if err != nil {
			return err
		}
		code = s.parseCode(response)
	}

	if code != 250 {
		return fmt.Errorf("MAIL FROM rejected with code %d: %s", code, response)
	}

	return nil
}

// RcptTo sends RCPT TO command and returns the result
func (s *SMTPConnection) RcptTo(email string) (int, string, error) {
	response, err := s.sendCommand(fmt.Sprintf("RCPT TO:<%s>", email))
	if err != nil {
		return 0, "", err
	}

	code := s.parseCode(response)
	return code, strings.TrimSpace(response), nil
}

// Reset sends RSET command
func (s *SMTPConnection) Reset() error {
	_, err := s.sendCommand("RSET")
	return err
}

// Quit sends QUIT command and closes connection
func (s *SMTPConnection) Quit() {
	if s.conn != nil {
		s.sendCommand("QUIT") //nolint:errcheck // best-effort on close
		s.conn.Close()
	}
}

// Close closes the connection without QUIT
func (s *SMTPConnection) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}

// IsConnected returns true if connection is active
func (s *SMTPConnection) IsConnected() bool {
	return s.conn != nil
}

// SupportsTLS returns true if server supports STARTTLS
func (s *SMTPConnection) SupportsTLS() bool {
	return s.features["STARTTLS"]
}

// UsingTLS returns true if connection is using TLS
func (s *SMTPConnection) UsingTLS() bool {
	return s.useTLS
}

// sendCommand sends a command and reads response
func (s *SMTPConnection) sendCommand(cmd string) (string, error) {
	log := debug.GetLogger()

	log.SMTPSend(cmd)

	s.conn.SetDeadline(time.Now().Add(s.config.Timeout))

	_, err := fmt.Fprintf(s.conn, "%s\r\n", cmd)
	if err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	response, err := s.readResponse()
	if err != nil {
		return "", err
	}

	log.SMTPRecv(strings.TrimSpace(response))

	return response, nil
}

// readResponse reads a (possibly multi-line) SMTP response.
// It enforces maxSMTPResponseSize to protect against memory exhaustion.
func (s *SMTPConnection) readResponse() (string, error) {
	var response strings.Builder
	totalBytes := 0

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return response.String(), fmt.Errorf("failed to read response: %w", err)
		}

		totalBytes += len(line)
		if totalBytes > maxSMTPResponseSize {
			return "", fmt.Errorf("SMTP response exceeded maximum size (%d bytes) — possible attack", maxSMTPResponseSize)
		}

		response.WriteString(line)

		// Check if this is the last line: 4th char is space, not hyphen
		if len(line) >= 4 && line[3] == ' ' {
			break
		}
	}

	return response.String(), nil
}

// parseCode extracts the numeric code from an SMTP response
func (s *SMTPConnection) parseCode(response string) int {
	if len(response) < 3 {
		return 0
	}
	code, err := strconv.Atoi(response[:3])
	if err != nil {
		return 0
	}
	return code
}

// tlsVersionString returns a human-readable TLS version string
func tlsVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("0x%04x", version)
	}
}

// GenerateRandomEmail generates a cryptographically random non-existent email
// for catch-all domain detection. Uses crypto/rand (not math/rand).
func GenerateRandomEmail(domain string) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const length = 16

	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use time-based suffix (should never happen in practice)
		return fmt.Sprintf("emailverify_test_%d@%s", time.Now().UnixNano(), domain)
	}

	result := make([]byte, length)
	for i, v := range b {
		result[i] = charset[int(v)%len(charset)]
	}
	return fmt.Sprintf("emailverify_test_%s@%s", string(result), domain)
}

// VerifyEmail performs SMTP verification for a single email
func VerifyEmail(config *SMTPConfig, email string, checkCatchAll bool) (*Result, error) {
	log := debug.GetLogger()
	result := NewResult(email)

	totalTimer := log.StartTimer("VERIFY", fmt.Sprintf("Verifying %s", email))
	defer func() {
		result.LatencyMs = totalTimer.Elapsed().Milliseconds()
		totalTimer.Stop()
	}()

	// Create and connect
	smtp := NewSMTPConnection(config)
	defer smtp.Close()

	if err := smtp.Connect(); err != nil {
		result.SetError(err)
		return result, err
	}

	if err := smtp.EHLO(); err != nil {
		result.SetError(err)
		return result, err
	}

	// Try STARTTLS if available
	if smtp.SupportsTLS() {
		if err := smtp.StartTLS(); err != nil {
			log.Detail("SMTP", "STARTTLS failed, continuing without TLS: %v", err)
		}
	}
	result.TLSUsed = smtp.UsingTLS()

	// MAIL FROM
	if err := smtp.MailFrom(config.FromAddress); err != nil {
		result.SetError(err)
		return result, err
	}

	// RCPT TO — the actual mailbox probe
	code, response, err := smtp.RcptTo(email)
	if err != nil {
		result.SetError(err)
		return result, err
	}

	result.StatusCode = code
	result.SMTPResponse = response

	switch {
	case code == 250 || code == 251:
		result.SetValid(code, response)
		log.Success("VERIFY", "Email VALID: %s (code: %d)", email, code)

	case code == 252:
		result.SetUnknown("Server cannot verify but will attempt delivery")
		log.Info("VERIFY", "Email UNKNOWN: %s (code: %d)", email, code)

	case code >= 550 && code <= 559:
		reason := parseRejectionReason(response)
		result.SetInvalid(code, response, reason)
		log.Info("VERIFY", "Email INVALID: %s (code: %d, reason: %s)", email, code, reason)

	case code >= 450 && code <= 459:
		result.SetUnknown("Temporary failure: " + response)
		log.Info("VERIFY", "Email TEMP ERROR: %s (code: %d)", email, code)

	default:
		result.SetUnknown(fmt.Sprintf("Unexpected code %d: %s", code, response))
		log.Info("VERIFY", "Email UNKNOWN: %s (code: %d)", email, code)
	}

	// Catch-all detection: only when email was valid and caller requested it
	if checkCatchAll && result.Status == StatusValid {
		if err := smtp.Reset(); err == nil {
			if err := smtp.MailFrom(config.FromAddress); err == nil {
				randomEmail := GenerateRandomEmail(result.Domain)
				log.Detail("CATCHALL", "Testing with random email: %s", randomEmail)

				catchCode, _, _ := smtp.RcptTo(randomEmail)
				result.CatchAllChecked = true

				if catchCode == 250 || catchCode == 251 {
					result.CatchAll = true
					result.SetRisky("Domain accepts all emails (catch-all)")
					log.Info("CATCHALL", "Domain is catch-all: %s", result.Domain)
				} else {
					log.Detail("CATCHALL", "Domain is NOT catch-all (random email rejected with %d)", catchCode)
				}
			}
		}
	}

	return result, nil
}

// parseRejectionReason extracts a human-readable reason from an SMTP rejection response
func parseRejectionReason(response string) string {
	r := strings.ToLower(response)

	switch {
	case strings.Contains(r, "user unknown") || strings.Contains(r, "does not exist"):
		return "User does not exist"
	case strings.Contains(r, "mailbox not found"):
		return "Mailbox not found"
	case strings.Contains(r, "recipient rejected"):
		return "Recipient rejected"
	case strings.Contains(r, "no such user"):
		return "No such user"
	case strings.Contains(r, "invalid recipient"):
		return "Invalid recipient"
	case strings.Contains(r, "disabled"):
		return "Mailbox disabled"
	case strings.Contains(r, "over quota"):
		return "Mailbox over quota"
	default:
		return "Recipient rejected"
	}
}
