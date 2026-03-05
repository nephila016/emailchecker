package verifier

import (
	"fmt"
	"time"

	"github.com/nephila016/emailchecker/internal/classifier"
	"github.com/nephila016/emailchecker/internal/debug"
)

// Config holds verifier configuration
type Config struct {
	// Connection settings
	CustomHost  string
	Port        int
	Timeout     time.Duration
	FromAddress string
	HELODomain  string

	// Verification options
	SkipSMTP      bool
	CheckCatchAll bool
	SkipTLSVerify bool

	// Classification options
	CheckDisposable   bool
	CheckRole         bool
	CheckFreeProvider bool

	// Retry settings
	// MaxMXFallback is how many MX servers to try before giving up (0 = try all)
	MaxMXFallback int
}

// DefaultConfig returns default verifier configuration
func DefaultConfig() *Config {
	return &Config{
		Port:              25,
		Timeout:           15 * time.Second,
		FromAddress:       "test@gmail.com",
		HELODomain:        "mail.verification-check.com",
		CheckCatchAll:     false,
		SkipTLSVerify:     false, // Secure default: verify TLS certificates
		CheckDisposable:   true,
		CheckRole:         true,
		CheckFreeProvider: true,
		MaxMXFallback:     3, // Try up to 3 MX servers before giving up
	}
}

// Verifier performs email verification
type Verifier struct {
	config *Config
}

// New creates a new Verifier
func New(config *Config) *Verifier {
	if config == nil {
		config = DefaultConfig()
	}
	return &Verifier{config: config}
}

// Verify performs complete email verification
func (v *Verifier) Verify(email string) *Result {
	log := debug.GetLogger()
	result := NewResult(email)

	totalTimer := log.StartTimer("VERIFY", fmt.Sprintf("Full verification for %s", email))
	defer func() {
		result.LatencyMs = totalTimer.Elapsed().Milliseconds()
		totalTimer.Stop()
	}()

	// Layer 1: Syntax validation
	log.Info("VERIFY", "Layer 1: Syntax validation")
	localPart, domain, valid := ValidateSyntax(email)
	result.SyntaxValid = valid
	result.LocalPart = localPart
	result.Domain = domain

	if !valid {
		result.SetInvalid(0, "", "Invalid email syntax")
		return result
	}

	// Optional: hint about likely typos
	if suggestion := SuggestTypoFix(domain); suggestion != "" {
		log.Info("VERIFY", "Possible typo detected: %s -> %s", domain, suggestion)
	}

	// Layer 2: Domain / MX lookup
	log.Info("VERIFY", "Layer 2: Domain/MX validation")
	dnsResult, err := LookupMX(domain, v.config.Timeout)
	if err != nil {
		result.SetInvalid(0, "", fmt.Sprintf("domain %s lookup failed: %v", domain, err))
		return result
	}

	result.HasMX = dnsResult.HasMX
	result.MXRecords = dnsResult.GetMXHosts()
	if len(result.MXRecords) > 0 {
		result.MXHost = result.MXRecords[0]
	}

	// Layer 3: Pre-SMTP classification
	log.Info("VERIFY", "Layer 3: Pre-SMTP classification")

	if v.config.CheckDisposable {
		result.Disposable = classifier.IsDisposable(domain)
		if result.Disposable {
			log.Info("CLASSIFY", "Disposable email detected: %s", domain)
		}
	}

	if v.config.CheckRole {
		result.RoleAccount = classifier.IsRoleAccount(localPart)
		if result.RoleAccount {
			log.Info("CLASSIFY", "Role account detected: %s", localPart)
		}
	}

	if v.config.CheckFreeProvider {
		result.FreeProvider = classifier.IsFreeProvider(domain)
		if result.FreeProvider {
			log.Detail("CLASSIFY", "Free provider: %s", domain)
		}
	}

	// Skip SMTP if configured
	if v.config.SkipSMTP {
		log.Info("VERIFY", "SMTP verification skipped (--skip-smtp)")
		result.SetUnknown("SMTP verification skipped")
		result.ConfidenceScore = calculateConfidence(result)
		return result
	}

	// Layer 4: SMTP verification
	log.Info("VERIFY", "Layer 4: SMTP verification")

	// Use custom host if provided, otherwise walk MX records in priority order
	if v.config.CustomHost != "" {
		result.MXHost = v.config.CustomHost
		smtpResult, smtpErr := v.trySMTP(v.config.CustomHost, email)
		v.copySmtpResult(result, smtpResult, smtpErr)
	} else {
		if len(result.MXRecords) == 0 {
			result.SetInvalid(0, "", "No mail server found")
			return result
		}
		v.tryMXFallback(result, email)
	}

	// Final confidence score
	result.ConfidenceScore = calculateConfidence(result)
	return result
}

// tryMXFallback attempts SMTP verification against MX records in priority order.
// It stops at the first non-error result or when MaxMXFallback is reached.
func (v *Verifier) tryMXFallback(result *Result, email string) {
	log := debug.GetLogger()

	limit := len(result.MXRecords)
	if v.config.MaxMXFallback > 0 && v.config.MaxMXFallback < limit {
		limit = v.config.MaxMXFallback
	}

	for i := 0; i < limit; i++ {
		mxHost := result.MXRecords[i]
		result.MXHost = mxHost

		if i > 0 {
			log.Info("VERIFY", "Primary MX failed, trying fallback MX[%d]: %s", i, mxHost)
		}

		smtpResult, err := v.trySMTP(mxHost, email)
		v.copySmtpResult(result, smtpResult, err)

		// Stop if we got a definitive answer (not a connection/transport error)
		if result.Status != StatusError {
			return
		}
	}

	// All MX servers failed
	result.SetError(fmt.Errorf("all %d MX server(s) failed for %s; last SMTP error: %s", limit, email, result.Error))
	log.Error("VERIFY", "All %d MX server(s) failed for %s", limit, email)
}

// trySMTP performs SMTP verification against a single host
func (v *Verifier) trySMTP(host, email string) (*Result, error) {
	smtpConfig := &SMTPConfig{
		Host:          host,
		Port:          v.config.Port,
		Timeout:       v.config.Timeout,
		FromAddress:   v.config.FromAddress,
		HELODomain:    v.config.HELODomain,
		SkipTLSVerify: v.config.SkipTLSVerify,
	}
	return VerifyEmail(smtpConfig, email, v.config.CheckCatchAll)
}

// copySmtpResult copies SMTP result fields into the main result
func (v *Verifier) copySmtpResult(result *Result, smtpResult *Result, err error) {
	if err != nil {
		result.SetError(fmt.Errorf("SMTP verification failed for %s via %s:%d: %w", result.Email, result.MXHost, v.config.Port, err))
		return
	}
	if smtpResult == nil {
		result.SetError(fmt.Errorf("SMTP verification returned empty result for %s via %s:%d", result.Email, result.MXHost, v.config.Port))
		return
	}
	if smtpResult.Status == StatusError && smtpResult.Error != "" {
		result.SetError(fmt.Errorf("SMTP verification failed for %s via %s:%d: %s", result.Email, result.MXHost, v.config.Port, smtpResult.Error))
		return
	}
	result.Valid = smtpResult.Valid
	result.Status = smtpResult.Status
	result.StatusCode = smtpResult.StatusCode
	result.SMTPResponse = smtpResult.SMTPResponse
	result.Reason = smtpResult.Reason
	result.CatchAll = smtpResult.CatchAll
	result.CatchAllChecked = smtpResult.CatchAllChecked
	result.TLSUsed = smtpResult.TLSUsed
	result.SMTPSuccess = smtpResult.SMTPSuccess
	if smtpResult.Error != "" {
		result.Error = smtpResult.Error
	}
}

// VerifyBatch verifies multiple emails sequentially.
// For concurrent bulk verification use worker.Pool instead.
func (v *Verifier) VerifyBatch(emails []string) []*Result {
	results := make([]*Result, len(emails))
	for i, email := range emails {
		results[i] = v.Verify(email)
	}
	return results
}

// QuickCheck performs syntax and DNS check only (no SMTP).
// It is safe to call concurrently — it does NOT mutate the receiver's config.
func (v *Verifier) QuickCheck(email string) *Result {
	// Create an isolated copy of config to avoid a data race on SkipSMTP.
	cfgCopy := *v.config
	cfgCopy.SkipSMTP = true
	quickV := &Verifier{config: &cfgCopy}
	return quickV.Verify(email)
}

// CheckDomain checks domain-level information (MX, SPF, DMARC, classification)
func (v *Verifier) CheckDomain(domain string, checkSPF, checkDMARC bool) (*DomainResult, error) {
	log := debug.GetLogger()

	result := &DomainResult{
		Domain: domain,
	}

	log.Info("DOMAIN", "Checking MX records for %s", domain)
	dnsResult, err := LookupMX(domain, v.config.Timeout)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.HasMX = dnsResult.HasMX
	result.MXRecords = dnsResult.GetMXHosts()

	if checkSPF {
		log.Info("DOMAIN", "Checking SPF record")
		result.SPFRecord, result.HasSPF = LookupSPF(domain, v.config.Timeout)
	}

	if checkDMARC {
		log.Info("DOMAIN", "Checking DMARC record")
		result.DMARCRecord, result.HasDMARC = LookupDMARC(domain, v.config.Timeout)
	}

	result.IsDisposable = classifier.IsDisposable(domain)
	result.IsFreeProvider = classifier.IsFreeProvider(domain)

	return result, nil
}

// DomainResult contains domain-level check results
type DomainResult struct {
	Domain         string   `json:"domain"`
	HasMX          bool     `json:"has_mx"`
	MXRecords      []string `json:"mx_records"`
	HasSPF         bool     `json:"has_spf"`
	SPFRecord      string   `json:"spf_record,omitempty"`
	HasDMARC       bool     `json:"has_dmarc"`
	DMARCRecord    string   `json:"dmarc_record,omitempty"`
	IsCatchAll     bool     `json:"is_catch_all"`
	IsDisposable   bool     `json:"is_disposable"`
	IsFreeProvider bool     `json:"is_free_provider"`
	Error          string   `json:"error,omitempty"`
}
