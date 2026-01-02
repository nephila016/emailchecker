package verifier

import (
	"fmt"
	"time"

	"github.com/yourusername/emailverify/internal/classifier"
	"github.com/yourusername/emailverify/internal/debug"
)

// Config holds verifier configuration
type Config struct {
	// Connection settings
	CustomHost    string
	Port          int
	Timeout       time.Duration
	FromAddress   string
	HELODomain    string

	// Verification options
	SkipSMTP       bool
	CheckCatchAll  bool
	SkipTLSVerify  bool

	// Classification options
	CheckDisposable  bool
	CheckRole        bool
	CheckFreeProvider bool
}

// DefaultConfig returns default verifier configuration
func DefaultConfig() *Config {
	return &Config{
		Port:             25,
		Timeout:          15 * time.Second,
		FromAddress:      "test@gmail.com",
		HELODomain:       "mail.verification-check.com",
		CheckCatchAll:    false,
		SkipTLSVerify:    true,
		CheckDisposable:  true,
		CheckRole:        true,
		CheckFreeProvider: true,
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
	}()

	// Layer 1: Syntax validation
	log.Info("VERIFY", "Layer 1: Syntax validation")
	localPart, domain, valid := ValidateSyntax(email)
	result.SyntaxValid = valid
	result.LocalPart = localPart
	result.Domain = domain

	if !valid {
		result.SetInvalid(0, "", "Invalid email syntax")
		totalTimer.Stop()
		return result
	}

	// Check for typos
	if suggestion := SuggestTypoFix(domain); suggestion != "" {
		log.Info("VERIFY", "Possible typo detected: %s -> %s", domain, suggestion)
	}

	// Layer 2: Domain checks
	log.Info("VERIFY", "Layer 2: Domain/MX validation")
	dnsResult, err := LookupMX(domain, v.config.Timeout)
	if err != nil {
		result.SetInvalid(0, "", fmt.Sprintf("Domain error: %v", err))
		totalTimer.Stop()
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
		totalTimer.Stop()
		return result
	}

	// Layer 4: SMTP verification
	log.Info("VERIFY", "Layer 4: SMTP verification")

	// Determine SMTP host
	smtpHost := v.config.CustomHost
	if smtpHost == "" {
		if len(result.MXRecords) == 0 {
			result.SetInvalid(0, "", "No mail server found")
			totalTimer.Stop()
			return result
		}
		smtpHost = result.MXRecords[0]
	}

	// Configure SMTP
	smtpConfig := &SMTPConfig{
		Host:          smtpHost,
		Port:          v.config.Port,
		Timeout:       v.config.Timeout,
		FromAddress:   v.config.FromAddress,
		HELODomain:    v.config.HELODomain,
		SkipTLSVerify: v.config.SkipTLSVerify,
	}

	// Perform SMTP verification
	smtpResult, err := VerifyEmail(smtpConfig, email, v.config.CheckCatchAll)
	if err != nil {
		log.Error("VERIFY", "SMTP verification error: %v", err)
		result.SetError(err)
		totalTimer.Stop()
		return result
	}

	// Copy SMTP results
	result.Valid = smtpResult.Valid
	result.Status = smtpResult.Status
	result.StatusCode = smtpResult.StatusCode
	result.SMTPResponse = smtpResult.SMTPResponse
	result.Reason = smtpResult.Reason
	result.CatchAll = smtpResult.CatchAll
	result.CatchAllChecked = smtpResult.CatchAllChecked
	result.TLSUsed = smtpResult.TLSUsed
	result.SMTPSuccess = smtpResult.SMTPSuccess

	// Recalculate confidence with all data
	result.ConfidenceScore = calculateConfidence(result)

	totalTimer.Stop()
	return result
}

// VerifyBatch verifies multiple emails (sequential)
func (v *Verifier) VerifyBatch(emails []string) []*Result {
	results := make([]*Result, len(emails))
	for i, email := range emails {
		results[i] = v.Verify(email)
	}
	return results
}

// QuickCheck performs syntax and DNS check only (no SMTP)
func (v *Verifier) QuickCheck(email string) *Result {
	originalSkip := v.config.SkipSMTP
	v.config.SkipSMTP = true
	result := v.Verify(email)
	v.config.SkipSMTP = originalSkip
	return result
}

// CheckDomain checks domain-level information
func (v *Verifier) CheckDomain(domain string) (*DomainResult, error) {
	log := debug.GetLogger()

	result := &DomainResult{
		Domain: domain,
	}

	// MX lookup
	log.Info("DOMAIN", "Checking MX records for %s", domain)
	dnsResult, err := LookupMX(domain, v.config.Timeout)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.HasMX = dnsResult.HasMX
	result.MXRecords = dnsResult.GetMXHosts()

	// SPF check
	log.Info("DOMAIN", "Checking SPF record")
	result.SPFRecord, result.HasSPF = LookupSPF(domain, v.config.Timeout)

	// DMARC check
	log.Info("DOMAIN", "Checking DMARC record")
	result.DMARCRecord, result.HasDMARC = LookupDMARC(domain, v.config.Timeout)

	// Classification
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
