package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/yourusername/emailverify/internal/debug"
	"github.com/yourusername/emailverify/internal/output"
	"github.com/yourusername/emailverify/internal/verifier"
)

var (
	checkIP          string
	checkPort        int
	checkTimeout     int
	checkFromAddress string
	checkHELO        string
	checkSkipSMTP    bool
	checkOutput      string
	checkJSON        bool
	checkCatchAll    bool
)

var checkCmd = &cobra.Command{
	Use:   "check <email>",
	Short: "Verify a single email address",
	Long: `Verify a single email address using SMTP RCPT TO method.

The verification process includes:
  1. Syntax validation (RFC 5322)
  2. Domain/MX record lookup
  3. Classification (disposable, role, free provider)
  4. SMTP verification (optional)
  5. Catch-all detection (optional)

Examples:
  emailverify check user@example.com
  emailverify check user@example.com -i 192.168.1.100 -p 25
  emailverify check user@example.com --skip-smtp
  emailverify check user@example.com --json`,
	Args: cobra.ExactArgs(1),
	RunE: runCheck,
}

func init() {
	rootCmd.AddCommand(checkCmd)

	checkCmd.Flags().StringVarP(&checkIP, "ip", "i", "", "Custom SMTP server IP/hostname")
	checkCmd.Flags().IntVarP(&checkPort, "port", "p", 25, "SMTP port")
	checkCmd.Flags().IntVarP(&checkTimeout, "timeout", "t", 15, "Connection timeout in seconds")
	checkCmd.Flags().StringVar(&checkFromAddress, "from", "test@gmail.com", "MAIL FROM address")
	checkCmd.Flags().StringVar(&checkHELO, "helo", "mail.verification-check.com", "EHLO domain")
	checkCmd.Flags().BoolVar(&checkSkipSMTP, "skip-smtp", false, "Skip SMTP verification")
	checkCmd.Flags().StringVarP(&checkOutput, "output", "o", "", "Output file")
	checkCmd.Flags().BoolVar(&checkJSON, "json", false, "Output as JSON to stdout")
	checkCmd.Flags().BoolVar(&checkCatchAll, "catch-all", false, "Check for catch-all domain")
}

func runCheck(cmd *cobra.Command, args []string) error {
	email := args[0]
	log := debug.GetLogger()

	log.Info("CHECK", "Verifying email: %s", email)

	// Create verifier config
	config := &verifier.Config{
		CustomHost:      checkIP,
		Port:            checkPort,
		Timeout:         time.Duration(checkTimeout) * time.Second,
		FromAddress:     checkFromAddress,
		HELODomain:      checkHELO,
		SkipSMTP:        checkSkipSMTP,
		CheckCatchAll:   checkCatchAll,
		CheckDisposable: true,
		CheckRole:       true,
		CheckFreeProvider: true,
	}

	// Create verifier and run
	v := verifier.New(config)
	result := v.Verify(email)

	// Output
	if checkJSON {
		return outputJSON(result)
	}

	if checkOutput != "" {
		return outputToFile(result, checkOutput)
	}

	return outputConsole(result)
}

func outputJSON(result *verifier.Result) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func outputToFile(result *verifier.Result, filename string) error {
	format := output.DetectFormat(filename)
	writer, err := output.NewWriter(filename, format)
	if err != nil {
		return err
	}
	defer writer.Close()

	if err := writer.Write(result); err != nil {
		return err
	}

	fmt.Printf("Result saved to: %s\n", filename)
	return nil
}

func outputConsole(result *verifier.Result) error {
	green := color.New(color.FgGreen, color.Bold)
	red := color.New(color.FgRed, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)
	cyan := color.New(color.FgCyan)
	white := color.New(color.FgWhite, color.Bold)

	fmt.Println()
	white.Printf("Email: %s\n", result.Email)
	fmt.Println()

	// Status
	fmt.Print("Status: ")
	switch result.Status {
	case verifier.StatusValid:
		green.Println("VALID")
	case verifier.StatusInvalid:
		red.Println("INVALID")
	case verifier.StatusRisky:
		yellow.Println("RISKY")
	case verifier.StatusUnknown:
		yellow.Println("UNKNOWN")
	case verifier.StatusError:
		red.Println("ERROR")
	}

	if result.Reason != "" {
		fmt.Printf("Reason: %s\n", result.Reason)
	}

	fmt.Println()
	cyan.Println("Details:")

	// Syntax
	if result.SyntaxValid {
		fmt.Printf("  Syntax:       %s\n", green.Sprint("Valid"))
	} else {
		fmt.Printf("  Syntax:       %s\n", red.Sprint("Invalid"))
	}

	// Domain
	fmt.Printf("  Domain:       %s\n", result.Domain)

	// MX Records
	if result.HasMX {
		fmt.Printf("  MX Records:   %s\n", green.Sprint("Found"))
		if result.MXHost != "" {
			fmt.Printf("  Primary MX:   %s\n", result.MXHost)
		}
	} else {
		fmt.Printf("  MX Records:   %s\n", red.Sprint("Not found"))
	}

	// SMTP
	if result.SMTPSuccess {
		fmt.Printf("  SMTP Check:   %s (code: %d)\n", green.Sprint("Success"), result.StatusCode)
	} else if result.StatusCode > 0 {
		fmt.Printf("  SMTP Check:   %s (code: %d)\n", red.Sprint("Failed"), result.StatusCode)
	} else {
		fmt.Printf("  SMTP Check:   %s\n", yellow.Sprint("Not performed"))
	}

	// TLS
	if result.TLSUsed {
		fmt.Printf("  TLS:          %s\n", green.Sprint("Yes"))
	}

	fmt.Println()
	cyan.Println("Classification:")

	// Disposable
	if result.Disposable {
		fmt.Printf("  Disposable:   %s\n", red.Sprint("Yes"))
	} else {
		fmt.Printf("  Disposable:   %s\n", green.Sprint("No"))
	}

	// Role account
	if result.RoleAccount {
		fmt.Printf("  Role Account: %s\n", yellow.Sprint("Yes"))
	} else {
		fmt.Printf("  Role Account: %s\n", green.Sprint("No"))
	}

	// Free provider
	if result.FreeProvider {
		fmt.Printf("  Free Provider: %s\n", yellow.Sprint("Yes"))
	} else {
		fmt.Printf("  Free Provider: %s\n", green.Sprint("No"))
	}

	// Catch-all
	if result.CatchAllChecked {
		if result.CatchAll {
			fmt.Printf("  Catch-All:    %s\n", yellow.Sprint("Yes (risky)"))
		} else {
			fmt.Printf("  Catch-All:    %s\n", green.Sprint("No"))
		}
	}

	fmt.Println()
	fmt.Printf("Confidence Score: %d/100\n", result.ConfidenceScore)
	fmt.Printf("Latency: %dms\n", result.LatencyMs)
	fmt.Println()

	return nil
}
