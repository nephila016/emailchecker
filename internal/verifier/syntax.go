package verifier

import (
	"regexp"
	"strings"

	"github.com/nephila016/emailchecker/internal/debug"
)

// RFC 5322 compliant email regex (simplified but effective)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

// ValidateSyntax checks if the email has valid syntax
func ValidateSyntax(email string) (localPart, domain string, valid bool) {
	log := debug.GetLogger()

	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	log.Info("SYNTAX", "Validating syntax for: %s", email)

	// Basic checks
	if email == "" {
		log.Detail("SYNTAX", "Empty email address")
		return "", "", false
	}

	if len(email) > 254 {
		log.Detail("SYNTAX", "Email too long: %d chars (max 254)", len(email))
		return "", "", false
	}

	// Must contain exactly one @
	atCount := strings.Count(email, "@")
	if atCount != 1 {
		log.Detail("SYNTAX", "Invalid @ count: %d (must be 1)", atCount)
		return "", "", false
	}

	// Split into local and domain parts
	parts := strings.SplitN(email, "@", 2)
	localPart = parts[0]
	domain = parts[1]

	// Validate local part
	if localPart == "" {
		log.Detail("SYNTAX", "Empty local part")
		return "", "", false
	}

	if len(localPart) > 64 {
		log.Detail("SYNTAX", "Local part too long: %d chars (max 64)", len(localPart))
		return "", "", false
	}

	// Local part cannot start or end with a dot
	if strings.HasPrefix(localPart, ".") || strings.HasSuffix(localPart, ".") {
		log.Detail("SYNTAX", "Local part starts/ends with dot")
		return "", "", false
	}

	// No consecutive dots in local part
	if strings.Contains(localPart, "..") {
		log.Detail("SYNTAX", "Consecutive dots in local part")
		return "", "", false
	}

	// Validate domain
	if domain == "" {
		log.Detail("SYNTAX", "Empty domain")
		return "", "", false
	}

	if len(domain) > 253 {
		log.Detail("SYNTAX", "Domain too long: %d chars (max 253)", len(domain))
		return "", "", false
	}

	// Domain must contain at least one dot
	if !strings.Contains(domain, ".") {
		log.Detail("SYNTAX", "Domain has no dot")
		return "", "", false
	}

	// Domain cannot start or end with a dot or hyphen
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") ||
		strings.HasPrefix(domain, "-") || strings.HasSuffix(domain, "-") {
		log.Detail("SYNTAX", "Domain starts/ends with invalid char")
		return "", "", false
	}

	// No consecutive dots in domain
	if strings.Contains(domain, "..") {
		log.Detail("SYNTAX", "Consecutive dots in domain")
		return "", "", false
	}

	// Validate each domain label
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			log.Detail("SYNTAX", "Invalid label length: %d", len(label))
			return "", "", false
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			log.Detail("SYNTAX", "Label starts/ends with hyphen: %s", label)
			return "", "", false
		}
	}

	// TLD validation (must be at least 2 chars, only letters)
	tld := labels[len(labels)-1]
	if len(tld) < 2 {
		log.Detail("SYNTAX", "TLD too short: %s", tld)
		return "", "", false
	}

	// Check TLD is letters only (no numbers)
	for _, c := range tld {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			log.Detail("SYNTAX", "TLD contains non-letter: %s", tld)
			return "", "", false
		}
	}

	// Final regex check
	if !emailRegex.MatchString(email) {
		log.Detail("SYNTAX", "Failed regex validation")
		return "", "", false
	}

	log.Success("SYNTAX", "Valid syntax - local: %s, domain: %s", localPart, domain)
	return localPart, domain, true
}

// NormalizeEmail normalizes an email address
func NormalizeEmail(email string) string {
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	return email
}

// SuggestTypoFix suggests corrections for common domain typos
func SuggestTypoFix(domain string) string {
	typoMap := map[string]string{
		// Gmail typos
		"gmial.com":   "gmail.com",
		"gmai.com":    "gmail.com",
		"gmaill.com":  "gmail.com",
		"gmail.co":    "gmail.com",
		"gmail.cm":    "gmail.com",
		"gamil.com":   "gmail.com",
		"gnail.com":   "gmail.com",
		"gmal.com":    "gmail.com",
		"gmeil.com":   "gmail.com",
		"g]mail.com":  "gmail.com",
		"gimail.com":  "gmail.com",

		// Yahoo typos
		"yaho.com":    "yahoo.com",
		"yahooo.com":  "yahoo.com",
		"yhoo.com":    "yahoo.com",
		"yahoo.co":    "yahoo.com",
		"yahoo.cm":    "yahoo.com",
		"yhaoo.com":   "yahoo.com",

		// Hotmail typos
		"hotmal.com":   "hotmail.com",
		"hotmial.com":  "hotmail.com",
		"hotmail.co":   "hotmail.com",
		"hotmail.cm":   "hotmail.com",
		"hotmaill.com": "hotmail.com",
		"homail.com":   "hotmail.com",
		"htmail.com":   "hotmail.com",

		// Outlook typos
		"outlok.com":   "outlook.com",
		"outloo.com":   "outlook.com",
		"outlook.co":   "outlook.com",
		"outllook.com": "outlook.com",

		// iCloud typos
		"iclod.com":  "icloud.com",
		"icould.com": "icloud.com",
		"icloud.co":  "icloud.com",

		// Common .com typos
		"protonmail.co": "protonmail.com",
		"aol.co":        "aol.com",
	}

	domain = strings.ToLower(domain)
	if suggestion, ok := typoMap[domain]; ok {
		return suggestion
	}
	return ""
}
