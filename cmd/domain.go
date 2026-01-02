package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/nephila016/emailchecker/internal/debug"
	"github.com/nephila016/emailchecker/internal/verifier"
)

var (
	domainCheckCatchAll bool
	domainCheckSPF      bool
	domainCheckDMARC    bool
	domainJSON          bool
	domainTimeout       int
)

var domainCmd = &cobra.Command{
	Use:   "domain <domain>",
	Short: "Check domain-level information",
	Long: `Check domain-level information including MX records, SPF, DMARC,
and catch-all configuration.

Examples:
  emailverify domain example.com
  emailverify domain example.com --check-catchall
  emailverify domain example.com --check-spf --check-dmarc
  emailverify domain example.com --json`,
	Args: cobra.ExactArgs(1),
	RunE: runDomain,
}

func init() {
	rootCmd.AddCommand(domainCmd)

	domainCmd.Flags().BoolVar(&domainCheckCatchAll, "check-catchall", false, "Check for catch-all configuration")
	domainCmd.Flags().BoolVar(&domainCheckSPF, "check-spf", false, "Check SPF record")
	domainCmd.Flags().BoolVar(&domainCheckDMARC, "check-dmarc", false, "Check DMARC record")
	domainCmd.Flags().BoolVar(&domainJSON, "json", false, "Output as JSON")
	domainCmd.Flags().IntVarP(&domainTimeout, "timeout", "t", 15, "Timeout in seconds")
}

func runDomain(cmd *cobra.Command, args []string) error {
	domain := args[0]
	log := debug.GetLogger()

	log.Info("DOMAIN", "Checking domain: %s", domain)

	config := &verifier.Config{
		Timeout: time.Duration(domainTimeout) * time.Second,
	}
	v := verifier.New(config)

	result, err := v.CheckDomain(domain)
	if err != nil {
		return err
	}

	// Check SPF if requested
	if domainCheckSPF {
		result.SPFRecord, result.HasSPF = verifier.LookupSPF(domain, config.Timeout)
	}

	// Check DMARC if requested
	if domainCheckDMARC {
		result.DMARCRecord, result.HasDMARC = verifier.LookupDMARC(domain, config.Timeout)
	}

	// Check catch-all if requested
	if domainCheckCatchAll && result.HasMX {
		result.IsCatchAll = checkCatchAll(domain, result.MXRecords[0], config)
	}

	if domainJSON {
		return outputDomainJSON(result)
	}

	return outputDomainConsole(result)
}

func checkCatchAll(domain, mxHost string, config *verifier.Config) bool {
	log := debug.GetLogger()
	log.Info("CATCHALL", "Testing catch-all for %s via %s", domain, mxHost)

	// Generate random email
	randomEmail := verifier.GenerateRandomEmail(domain)

	smtpConfig := &verifier.SMTPConfig{
		Host:        mxHost,
		Port:        25,
		Timeout:     config.Timeout,
		FromAddress: "test@gmail.com",
		HELODomain:  "mail.verification-check.com",
	}

	result, err := verifier.VerifyEmail(smtpConfig, randomEmail, false)
	if err != nil {
		log.Error("CATCHALL", "Failed to check catch-all: %v", err)
		return false
	}

	isCatchAll := result.Status == verifier.StatusValid
	if isCatchAll {
		log.Info("CATCHALL", "Domain is catch-all (random email accepted)")
	} else {
		log.Info("CATCHALL", "Domain is NOT catch-all (random email rejected)")
	}

	return isCatchAll
}

func outputDomainJSON(result *verifier.DomainResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func outputDomainConsole(result *verifier.DomainResult) error {
	green := color.New(color.FgGreen, color.Bold)
	red := color.New(color.FgRed, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)
	cyan := color.New(color.FgCyan)
	white := color.New(color.FgWhite, color.Bold)

	fmt.Println()
	white.Printf("Domain: %s\n", result.Domain)
	fmt.Println()

	// MX Records
	cyan.Println("MX Records:")
	if result.HasMX {
		for i, mx := range result.MXRecords {
			fmt.Printf("  [%d] %s\n", i+1, mx)
		}
	} else {
		red.Println("  No MX records found")
	}
	fmt.Println()

	// Classification
	cyan.Println("Classification:")
	if result.IsDisposable {
		fmt.Printf("  Disposable:    %s\n", red.Sprint("Yes"))
	} else {
		fmt.Printf("  Disposable:    %s\n", green.Sprint("No"))
	}

	if result.IsFreeProvider {
		fmt.Printf("  Free Provider: %s\n", yellow.Sprint("Yes"))
	} else {
		fmt.Printf("  Free Provider: %s\n", green.Sprint("No"))
	}

	if domainCheckCatchAll {
		if result.IsCatchAll {
			fmt.Printf("  Catch-All:     %s\n", yellow.Sprint("Yes"))
		} else {
			fmt.Printf("  Catch-All:     %s\n", green.Sprint("No"))
		}
	}
	fmt.Println()

	// SPF
	if domainCheckSPF {
		cyan.Println("SPF Record:")
		if result.HasSPF {
			fmt.Printf("  %s\n", result.SPFRecord)
		} else {
			yellow.Println("  No SPF record found")
		}
		fmt.Println()
	}

	// DMARC
	if domainCheckDMARC {
		cyan.Println("DMARC Record:")
		if result.HasDMARC {
			fmt.Printf("  %s\n", result.DMARCRecord)
		} else {
			yellow.Println("  No DMARC record found")
		}
		fmt.Println()
	}

	return nil
}
