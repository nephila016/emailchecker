package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/nephila016/emailchecker/internal/debug"
	"github.com/nephila016/emailchecker/internal/output"
	"github.com/nephila016/emailchecker/internal/verifier"
	"github.com/nephila016/emailchecker/internal/worker"
)

var (
	bulkFile           string
	bulkIP             string
	bulkPort           int
	bulkOutput         string
	bulkWorkers        int
	bulkDelay          float64
	bulkJitter         float64
	bulkTimeout        int
	bulkFromAddress    string
	bulkHELO           string
	bulkHealthEmail    string
	bulkHealthInterval int
	bulkReconnect      int
	bulkSkipSMTP       bool
	bulkResume         bool
	bulkProxy          string
	bulkCatchAll       bool
)

var bulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Verify multiple emails from a file",
	Long: `Verify multiple email addresses from an input file using concurrent workers.

Features:
  - Concurrent verification with configurable workers
  - Rate limiting with delay and jitter
  - Health checks with known-valid email
  - Progress bar and statistics
  - Incremental saving
  - Graceful shutdown on Ctrl+C
  - Automatic MX fallback (tries secondary MX if primary is down)
  - Duplicate email removal

Examples:
  emailverify bulk -f emails.txt -o results.csv
  emailverify bulk -f emails.txt -i mail.example.com -p 25 -w 5
  emailverify bulk -f emails.txt --health-email info@example.com
  emailverify bulk -f emails.txt -d 3 --jitter 2 -o results.json`,
	RunE: runBulk,
}

func init() {
	rootCmd.AddCommand(bulkCmd)

	bulkCmd.Flags().StringVarP(&bulkFile, "file", "f", "", "Input file with emails (required)")
	bulkCmd.Flags().StringVarP(&bulkIP, "ip", "i", "", "Custom SMTP server IP/hostname")
	bulkCmd.Flags().IntVarP(&bulkPort, "port", "p", 25, "SMTP port")
	bulkCmd.Flags().StringVarP(&bulkOutput, "output", "o", "results.csv", "Output file")
	bulkCmd.Flags().IntVarP(&bulkWorkers, "workers", "w", 3, "Number of concurrent workers")
	bulkCmd.Flags().Float64VarP(&bulkDelay, "delay", "d", 2.0, "Delay between checks (seconds)")
	bulkCmd.Flags().Float64Var(&bulkJitter, "jitter", 1.0, "Random jitter added to delay (seconds)")
	bulkCmd.Flags().IntVarP(&bulkTimeout, "timeout", "t", 15, "Connection timeout (seconds)")
	bulkCmd.Flags().StringVar(&bulkFromAddress, "from", "test@gmail.com", "MAIL FROM address")
	bulkCmd.Flags().StringVar(&bulkHELO, "helo", "mail.verification-check.com", "EHLO domain")
	bulkCmd.Flags().StringVar(&bulkHealthEmail, "health-email", "", "Known-valid email for health checks")
	bulkCmd.Flags().IntVar(&bulkHealthInterval, "health-interval", 10, "Health check every N emails")
	bulkCmd.Flags().IntVar(&bulkReconnect, "reconnect", 5, "Reconnect every N emails")
	bulkCmd.Flags().BoolVar(&bulkSkipSMTP, "skip-smtp", false, "Skip SMTP verification")
	bulkCmd.Flags().BoolVar(&bulkResume, "resume", false, "Resume from last position (not yet implemented)")
	bulkCmd.Flags().StringVar(&bulkProxy, "proxy", "", "SOCKS5 proxy socks5://[user:pass@]host:port (not yet implemented)")
	bulkCmd.Flags().BoolVar(&bulkCatchAll, "catch-all", false, "Check for catch-all domains")

	bulkCmd.MarkFlagRequired("file")
}

func runBulk(cmd *cobra.Command, args []string) error {
	log := debug.GetLogger()
	startTime := time.Now()

	// Warn about flags that are accepted but not yet implemented
	if bulkProxy != "" {
		color.New(color.FgYellow).Fprintln(os.Stderr,
			"Warning: --proxy is not yet implemented and will be ignored")
	}
	if bulkResume {
		color.New(color.FgYellow).Fprintln(os.Stderr,
			"Warning: --resume is not yet implemented and will be ignored")
	}

	// Load and deduplicate emails
	emails, duplicates, err := loadEmails(bulkFile)
	if err != nil {
		return err
	}

	if len(emails) == 0 {
		return fmt.Errorf("no emails found in %s", bulkFile)
	}

	if !quiet {
		printBulkSettings(len(emails), duplicates)
	}

	// Initial health check
	if bulkHealthEmail != "" {
		if !runInitialHealthCheck() {
			return fmt.Errorf("initial health check failed")
		}
	}

	// Build verifier config
	config := &verifier.Config{
		CustomHost:        bulkIP,
		Port:              bulkPort,
		Timeout:           time.Duration(bulkTimeout) * time.Second,
		FromAddress:       bulkFromAddress,
		HELODomain:        bulkHELO,
		SkipSMTP:          bulkSkipSMTP,
		CheckCatchAll:     bulkCatchAll,
		CheckDisposable:   true,
		CheckRole:         true,
		CheckFreeProvider: true,
	}
	v := verifier.New(config)

	// Open output writer
	format := output.DetectFormat(bulkOutput)
	writer, err := output.NewWriter(bulkOutput, format)
	if err != nil {
		return err
	}
	defer writer.Close()

	// Build worker pool
	poolConfig := &worker.PoolConfig{
		Workers:        bulkWorkers,
		Delay:          time.Duration(bulkDelay * float64(time.Second)),
		Jitter:         time.Duration(bulkJitter * float64(time.Second)),
		HealthEmail:    bulkHealthEmail,
		HealthInterval: bulkHealthInterval,
		BufferSize:     100,
	}
	pool := worker.NewPool(v, poolConfig)

	// Statistics (protected by mutex)
	var stats struct {
		sync.Mutex
		valid   int
		invalid int
		unknown int
		risky   int
		errors  int
	}

	// Progress bar
	var bar *progressbar.ProgressBar
	if !quiet {
		bar = progressbar.NewOptions(len(emails),
			progressbar.OptionSetDescription("Verifying"),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetItsString("emails"),
		)
	}

	// Graceful shutdown on Ctrl+C / SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigChan:
			fmt.Fprintln(os.Stderr, "\nShutting down gracefully...")
			cancel()
			pool.Stop()
		case <-ctx.Done():
		}
	}()

	// Per-result callback: update stats, write output, advance progress bar
	pool.SetCallbacks(
		func(result *verifier.Result) {
			stats.Lock()
			switch result.Status {
			case verifier.StatusValid:
				stats.valid++
			case verifier.StatusInvalid:
				stats.invalid++
			case verifier.StatusRisky:
				stats.risky++
			case verifier.StatusUnknown:
				stats.unknown++
			case verifier.StatusError:
				stats.errors++
			}
			stats.Unlock()

			if err := writer.Write(result); err != nil {
				log.Error("OUTPUT", "Failed to write result for %s: %v", result.Email, err)
			}
			writer.Flush()

			if bar != nil {
				bar.Add(1) //nolint:errcheck
			}

			log.Detail("RESULT", "%s: %s (code: %d)", result.Email, result.Status, result.StatusCode)
		},
		nil,
	)

	// Start workers
	pool.Start()

	// Feed jobs to the pool in a separate goroutine so we can also drain results
	go func() {
		for i, email := range emails {
			select {
			case <-ctx.Done():
				return
			default:
				pool.Submit(email, i)
			}
		}
		// All jobs submitted — signal workers to drain and exit
		pool.Close()
	}()

	// Drain result channel (actual processing happens in the callback)
	for range pool.Results() {
	}

	// Finalize
	if !quiet {
		if bar != nil {
			bar.Finish() //nolint:errcheck
		}
		printBulkSummary(&stats, len(emails), startTime)
	}

	fmt.Printf("\nResults saved to: %s\n", bulkOutput)
	return nil
}

// loadEmails reads emails from filename, skipping blanks and comments.
// It deduplicates (case-insensitive) and returns the unique list along with
// the number of duplicates removed.
func loadEmails(filename string) (emails []string, duplicates int, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	seen := make(map[string]struct{})

	scanner := bufio.NewScanner(f)
	// Allow lines up to 1 MB (handles extremely long lines gracefully)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key := strings.ToLower(line)
		if _, exists := seen[key]; exists {
			duplicates++
			continue
		}
		seen[key] = struct{}{}
		emails = append(emails, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, duplicates, fmt.Errorf("failed to read file: %w", err)
	}

	return emails, duplicates, nil
}

func runInitialHealthCheck() bool {
	log := debug.GetLogger()

	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	if !quiet {
		yellow.Println("\n--- Initial Health Check ---")
		fmt.Printf("Testing: %s\n", bulkHealthEmail)
	}

	config := &verifier.Config{
		CustomHost:  bulkIP,
		Port:        bulkPort,
		Timeout:     time.Duration(bulkTimeout) * time.Second,
		FromAddress: bulkFromAddress,
		HELODomain:  bulkHELO,
	}

	v := verifier.New(config)
	result := v.Verify(bulkHealthEmail)

	if result.Status == verifier.StatusValid {
		if !quiet {
			green.Printf("Health check PASSED: %s is valid\n\n", bulkHealthEmail)
		}
		log.Success("HEALTH", "Initial health check passed")
		return true
	}

	if !quiet {
		red.Printf("Health check FAILED: %s returned %s\n", bulkHealthEmail, result.Status)
		if result.Reason != "" {
			fmt.Printf("Reason: %s\n", result.Reason)
		}
	}
	log.Error("HEALTH", "Initial health check failed: %s", result.Status)
	return false
}

func printBulkSettings(count, duplicates int) {
	cyan := color.New(color.FgCyan)
	white := color.New(color.FgWhite, color.Bold)
	yellow := color.New(color.FgYellow)

	fmt.Println()
	cyan.Println("========================================")
	white.Println("       Email Verification Tool")
	cyan.Println("========================================")
	fmt.Println()

	fmt.Printf("Emails to verify:  %d\n", count)
	if duplicates > 0 {
		yellow.Printf("Duplicates removed: %d\n", duplicates)
	}
	if bulkIP != "" {
		fmt.Printf("Server:            %s:%d\n", bulkIP, bulkPort)
	} else {
		fmt.Printf("Server:            Auto (MX lookup with fallback)\n")
	}
	fmt.Printf("Workers:           %d\n", bulkWorkers)
	fmt.Printf("Delay:             %.1fs (+%.1fs jitter)\n", bulkDelay, bulkJitter)
	fmt.Printf("Timeout:           %ds\n", bulkTimeout)
	if bulkHealthEmail != "" {
		fmt.Printf("Health check:      Every %d emails\n", bulkHealthInterval)
		fmt.Printf("Health email:      %s\n", bulkHealthEmail)
	}
	fmt.Printf("Output:            %s\n", bulkOutput)
	fmt.Println()
}

func printBulkSummary(stats *struct {
	sync.Mutex
	valid   int
	invalid int
	unknown int
	risky   int
	errors  int
}, total int, startTime time.Time) {
	duration := time.Since(startTime)
	rate := float64(total) / duration.Seconds()

	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	cyan.Println("========================================")
	cyan.Println("              SUMMARY")
	cyan.Println("========================================")
	fmt.Println()

	stats.Lock()
	defer stats.Unlock()

	fmt.Printf("Total Verified:    %d\n", total)
	green.Printf("Valid:             %d\n", stats.valid)
	red.Printf("Invalid:           %d\n", stats.invalid)
	yellow.Printf("Unknown:           %d\n", stats.unknown)
	yellow.Printf("Risky:             %d\n", stats.risky)
	red.Printf("Errors:            %d\n", stats.errors)
	fmt.Println()
	fmt.Printf("Duration:          %s\n", duration.Round(time.Second))
	fmt.Printf("Rate:              %.2f emails/sec\n", rate)
}
