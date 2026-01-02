package verifier

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/nephila016/emailchecker/internal/debug"
)

// MXRecord represents an MX record with priority
type MXRecord struct {
	Host     string
	Priority uint16
}

// DNSResult contains DNS lookup results
type DNSResult struct {
	MXRecords  []MXRecord
	HasMX      bool
	SPFRecord  string
	HasSPF     bool
	DMARCRecord string
	HasDMARC   bool
	Error      error
}

// LookupMX performs MX record lookup for a domain
func LookupMX(domain string, timeout time.Duration) (*DNSResult, error) {
	log := debug.GetLogger()
	timer := log.StartTimer("DNS", fmt.Sprintf("MX lookup for %s", domain))
	defer timer.Stop()

	result := &DNSResult{
		MXRecords: []MXRecord{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resolver := &net.Resolver{
		PreferGo: true,
	}

	log.Detail("DNS", "Querying MX records for %s", domain)

	mxRecords, err := resolver.LookupMX(ctx, domain)
	if err != nil {
		// Check if it's a "no such host" error - domain might not have MX but could have A record
		if dnsErr, ok := err.(*net.DNSError); ok {
			if dnsErr.IsNotFound {
				log.Detail("DNS", "No MX records found for %s, checking A record", domain)
				// Try A record as fallback
				addrs, aErr := resolver.LookupHost(ctx, domain)
				if aErr == nil && len(addrs) > 0 {
					log.Detail("DNS", "Found A record, using domain as MX: %s", domain)
					result.MXRecords = []MXRecord{{Host: domain, Priority: 10}}
					result.HasMX = true
					return result, nil
				}
			}
		}
		log.Error("DNS", "MX lookup failed: %v", err)
		result.Error = fmt.Errorf("MX lookup failed: %w", err)
		return result, result.Error
	}

	if len(mxRecords) == 0 {
		log.Detail("DNS", "No MX records returned")
		result.Error = fmt.Errorf("no MX records found for %s", domain)
		return result, result.Error
	}

	// Convert and sort by priority
	for _, mx := range mxRecords {
		host := strings.TrimSuffix(mx.Host, ".")
		result.MXRecords = append(result.MXRecords, MXRecord{
			Host:     host,
			Priority: mx.Pref,
		})
		log.Detail("DNS", "  MX[%d]: %s (priority: %d)", len(result.MXRecords)-1, host, mx.Pref)
	}

	// Sort by priority (lower is better)
	sort.Slice(result.MXRecords, func(i, j int) bool {
		return result.MXRecords[i].Priority < result.MXRecords[j].Priority
	})

	result.HasMX = true
	log.Success("DNS", "Found %d MX record(s), primary: %s", len(result.MXRecords), result.MXRecords[0].Host)

	return result, nil
}

// LookupSPF retrieves SPF record for a domain
func LookupSPF(domain string, timeout time.Duration) (string, bool) {
	log := debug.GetLogger()
	log.Detail("DNS", "Querying SPF for %s", domain)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resolver := &net.Resolver{PreferGo: true}
	txtRecords, err := resolver.LookupTXT(ctx, domain)
	if err != nil {
		log.Detail("DNS", "SPF lookup failed: %v", err)
		return "", false
	}

	for _, txt := range txtRecords {
		if strings.HasPrefix(strings.ToLower(txt), "v=spf1") {
			log.Detail("DNS", "Found SPF: %s", txt)
			return txt, true
		}
	}

	log.Detail("DNS", "No SPF record found")
	return "", false
}

// LookupDMARC retrieves DMARC record for a domain
func LookupDMARC(domain string, timeout time.Duration) (string, bool) {
	log := debug.GetLogger()
	dmarcDomain := "_dmarc." + domain
	log.Detail("DNS", "Querying DMARC for %s", dmarcDomain)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resolver := &net.Resolver{PreferGo: true}
	txtRecords, err := resolver.LookupTXT(ctx, dmarcDomain)
	if err != nil {
		log.Detail("DNS", "DMARC lookup failed: %v", err)
		return "", false
	}

	for _, txt := range txtRecords {
		if strings.HasPrefix(strings.ToLower(txt), "v=dmarc1") {
			log.Detail("DNS", "Found DMARC: %s", txt)
			return txt, true
		}
	}

	log.Detail("DNS", "No DMARC record found")
	return "", false
}

// ResolveMXToIP resolves an MX hostname to IP addresses
func ResolveMXToIP(host string, timeout time.Duration) ([]string, error) {
	log := debug.GetLogger()
	log.Trace("DNS", "Resolving %s to IP", host)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resolver := &net.Resolver{PreferGo: true}
	addrs, err := resolver.LookupHost(ctx, host)
	if err != nil {
		log.Error("DNS", "Failed to resolve %s: %v", host, err)
		return nil, err
	}

	log.Trace("DNS", "Resolved %s to: %v", host, addrs)
	return addrs, nil
}

// GetMXHosts returns a list of MX hostnames sorted by priority
func (d *DNSResult) GetMXHosts() []string {
	hosts := make([]string, len(d.MXRecords))
	for i, mx := range d.MXRecords {
		hosts[i] = mx.Host
	}
	return hosts
}

// GetPrimaryMX returns the primary (lowest priority) MX host
func (d *DNSResult) GetPrimaryMX() string {
	if len(d.MXRecords) == 0 {
		return ""
	}
	return d.MXRecords[0].Host
}
