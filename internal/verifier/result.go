package verifier

import (
	"time"
)

// Status represents the verification status
type Status string

const (
	StatusValid    Status = "valid"
	StatusInvalid  Status = "invalid"
	StatusUnknown  Status = "unknown"
	StatusRisky    Status = "risky"
	StatusError    Status = "error"
)

// Result contains the complete verification result
type Result struct {
	Email           string    `json:"email"`
	Valid           bool      `json:"valid"`
	Status          Status    `json:"status"`
	StatusCode      int       `json:"status_code"`
	Reason          string    `json:"reason"`
	Disposable      bool      `json:"disposable"`
	RoleAccount     bool      `json:"role_account"`
	FreeProvider    bool      `json:"free_provider"`
	CatchAll        bool      `json:"catch_all"`
	CatchAllChecked bool      `json:"catch_all_checked"`
	MXRecords       []string  `json:"mx_records"`
	MXHost          string    `json:"mx_host"`
	SMTPResponse    string    `json:"smtp_response"`
	ConfidenceScore int       `json:"confidence_score"`
	VerifiedAt      time.Time `json:"verified_at"`
	LatencyMs       int64     `json:"latency_ms"`

	// Syntax check results
	SyntaxValid bool   `json:"syntax_valid"`
	LocalPart   string `json:"local_part"`
	Domain      string `json:"domain"`

	// Additional info
	HasMX       bool   `json:"has_mx"`
	SMTPSuccess bool   `json:"smtp_success"`
	TLSUsed     bool   `json:"tls_used"`
	Error       string `json:"error,omitempty"`
}

// NewResult creates a new Result with default values
func NewResult(email string) *Result {
	return &Result{
		Email:      email,
		Status:     StatusUnknown,
		VerifiedAt: time.Now(),
		MXRecords:  []string{},
	}
}

// SetValid marks the result as valid
func (r *Result) SetValid(code int, response string) {
	r.Valid = true
	r.Status = StatusValid
	r.StatusCode = code
	r.SMTPResponse = response
	r.SMTPSuccess = true
	r.ConfidenceScore = calculateConfidence(r)
}

// SetInvalid marks the result as invalid
func (r *Result) SetInvalid(code int, response, reason string) {
	r.Valid = false
	r.Status = StatusInvalid
	r.StatusCode = code
	r.SMTPResponse = response
	r.Reason = reason
	r.ConfidenceScore = calculateConfidence(r)
}

// SetUnknown marks the result as unknown
func (r *Result) SetUnknown(reason string) {
	r.Valid = false
	r.Status = StatusUnknown
	r.Reason = reason
	r.ConfidenceScore = calculateConfidence(r)
}

// SetRisky marks the result as risky (e.g., catch-all domain)
func (r *Result) SetRisky(reason string) {
	r.Valid = false
	r.Status = StatusRisky
	r.Reason = reason
	r.ConfidenceScore = calculateConfidence(r)
}

// SetError marks the result as error
func (r *Result) SetError(err error) {
	r.Valid = false
	r.Status = StatusError
	r.Error = err.Error()
	r.Reason = err.Error()
	r.ConfidenceScore = 0
}

// calculateConfidence calculates a confidence score 0-100
func calculateConfidence(r *Result) int {
	score := 0

	// Syntax valid adds base score
	if r.SyntaxValid {
		score += 10
	}

	// Has MX records
	if r.HasMX {
		score += 15
	}

	// SMTP verification results
	switch r.Status {
	case StatusValid:
		score += 60
		if r.StatusCode == 250 {
			score += 15
		}
	case StatusInvalid:
		score = 0 // Reset for invalid
	case StatusRisky:
		score += 30 // Catch-all or uncertain
	case StatusUnknown:
		score += 20
	}

	// Deductions
	if r.Disposable {
		score -= 20
	}
	if r.CatchAll {
		score -= 25
	}
	if r.RoleAccount {
		score -= 5
	}

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// IsDeliverable returns true if the email is likely deliverable
func (r *Result) IsDeliverable() bool {
	return r.Status == StatusValid || (r.Status == StatusRisky && r.CatchAll)
}

// Summary returns a human-readable summary
func (r *Result) Summary() string {
	switch r.Status {
	case StatusValid:
		return "Email is valid and deliverable"
	case StatusInvalid:
		return "Email does not exist: " + r.Reason
	case StatusRisky:
		if r.CatchAll {
			return "Domain accepts all emails (catch-all) - delivery uncertain"
		}
		return "Risky: " + r.Reason
	case StatusUnknown:
		return "Could not verify: " + r.Reason
	case StatusError:
		return "Error during verification: " + r.Error
	default:
		return "Unknown status"
	}
}
