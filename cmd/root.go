package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/nephila016/emailchecker/internal/debug"
)

var (
	cfgFile     string
	debugLevel  int
	debugFile   string
	quiet       bool
	noColor     bool
	version     string
	buildTime   string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "emailverify",
	Short: "Email verification tool using SMTP RCPT TO method",
	Long: `emailverify is a production-ready CLI tool for verifying email addresses
using the SMTP RCPT TO method without sending actual emails.

Features:
  - SMTP verification with STARTTLS support
  - Syntax and domain validation
  - Disposable email detection
  - Role account detection
  - Catch-all domain detection
  - Concurrent bulk verification
  - Multiple output formats (JSON, CSV, TXT)

Examples:
  emailverify check user@example.com
  emailverify bulk -f emails.txt -o results.csv
  emailverify domain example.com --check-catchall`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize debug logging
		level := debug.Level(debugLevel)
		if err := debug.Init(level, debugFile, !noColor); err != nil {
			return err
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		debug.Close()
	},
}

// Execute adds all child commands to the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// SetVersionInfo sets version information
func SetVersionInfo(v, bt string) {
	version = v
	buildTime = bt
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default $HOME/.emailverify.yaml)")
	rootCmd.PersistentFlags().CountVarP(&debugLevel, "debug", "d", "Enable debug mode (use -d, -dd, -ddd for more detail)")
	rootCmd.PersistentFlags().StringVar(&debugFile, "debug-file", "", "Write debug output to file")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode - minimal output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	// Bind flags to viper
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
		}
		viper.AddConfigPath(".")
		viper.SetConfigName(".emailverify")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()
	viper.ReadInConfig() // Ignore error if config doesn't exist
}
