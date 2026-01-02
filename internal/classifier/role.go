package classifier

import (
	"strings"
)

// Role-based email prefixes
var rolePrefixes = map[string]bool{
	// Administrative
	"admin":         true,
	"administrator": true,
	"postmaster":    true,
	"hostmaster":    true,
	"webmaster":     true,
	"root":          true,
	"sysadmin":      true,

	// Support
	"support":       true,
	"help":          true,
	"helpdesk":      true,
	"customerservice": true,
	"service":       true,
	"tech":          true,
	"technical":     true,

	// Contact/Info
	"info":          true,
	"information":   true,
	"contact":       true,
	"contactus":     true,
	"hello":         true,
	"hi":            true,
	"enquiry":       true,
	"enquiries":     true,
	"inquiry":       true,
	"feedback":      true,

	// Sales/Marketing
	"sales":         true,
	"marketing":     true,
	"press":         true,
	"media":         true,
	"pr":            true,
	"advertising":   true,
	"ads":           true,
	"partnerships":  true,
	"partner":       true,
	"business":      true,
	"biz":           true,

	// No-reply
	"noreply":       true,
	"no-reply":      true,
	"donotreply":    true,
	"do-not-reply":  true,
	"mailer-daemon": true,
	"mailerdaemon":  true,
	"daemon":        true,
	"bounce":        true,
	"bounces":       true,

	// Security/Abuse
	"abuse":         true,
	"security":      true,
	"spam":          true,
	"phishing":      true,
	"fraud":         true,
	"compliance":    true,
	"legal":         true,
	"privacy":       true,
	"dmca":          true,

	// Finance
	"billing":       true,
	"invoice":       true,
	"invoices":      true,
	"accounting":    true,
	"accounts":      true,
	"finance":       true,
	"payments":      true,
	"payroll":       true,

	// HR/Jobs
	"hr":            true,
	"humanresources": true,
	"recruiting":    true,
	"recruitment":   true,
	"jobs":          true,
	"careers":       true,
	"career":        true,
	"talent":        true,
	"resume":        true,
	"resumes":       true,
	"cv":            true,

	// Team/Department
	"team":          true,
	"staff":         true,
	"office":        true,
	"reception":     true,
	"all":           true,
	"everyone":      true,
	"company":       true,
	"group":         true,
	"dept":          true,
	"department":    true,

	// IT/Dev
	"it":            true,
	"dev":           true,
	"developer":     true,
	"developers":    true,
	"development":   true,
	"engineering":   true,
	"devops":        true,
	"ops":           true,
	"operations":    true,
	"network":       true,
	"sysops":        true,
	"noc":           true,

	// Orders/Shopping
	"orders":        true,
	"order":         true,
	"shop":          true,
	"store":         true,
	"checkout":      true,
	"shipping":      true,
	"delivery":      true,
	"returns":       true,
	"refund":        true,
	"refunds":       true,
	"fulfillment":   true,

	// Newsletters/Lists
	"news":          true,
	"newsletter":    true,
	"newsletters":   true,
	"updates":       true,
	"subscribe":     true,
	"subscriptions": true,
	"unsubscribe":   true,
	"list":          true,
	"lists":         true,
	"announce":      true,
	"announcements": true,
	"notifications": true,
	"alerts":        true,

	// Social
	"social":        true,
	"community":     true,
	"forum":         true,
	"blog":          true,

	// Misc
	"test":          true,
	"testing":       true,
	"demo":          true,
	"example":       true,
	"sample":        true,
	"null":          true,
	"void":          true,
	"nobody":        true,
	"www":           true,
	"ftp":           true,
	"mail":          true,
	"email":         true,
}

// IsRoleAccount checks if the local part indicates a role account
func IsRoleAccount(localPart string) bool {
	localPart = strings.ToLower(strings.TrimSpace(localPart))

	// Direct match
	if rolePrefixes[localPart] {
		return true
	}

	// Check if starts with role prefix followed by number or separator
	for prefix := range rolePrefixes {
		if strings.HasPrefix(localPart, prefix) {
			rest := strings.TrimPrefix(localPart, prefix)
			if rest == "" {
				return true
			}
			// Check for common separators or numbers
			if len(rest) > 0 {
				firstChar := rest[0]
				if firstChar == '-' || firstChar == '_' || firstChar == '.' ||
				   (firstChar >= '0' && firstChar <= '9') {
					return true
				}
			}
		}
	}

	return false
}

// GetRolePrefixCount returns the number of role prefixes
func GetRolePrefixCount() int {
	return len(rolePrefixes)
}
