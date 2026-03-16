// Package main provides the CLI entrypoint for Tegata.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/josh-wong/tegata/internal/errors"
	"github.com/spf13/cobra"
)

// version is set via -ldflags "-X main.version=..." at build time.
var version = "dev"

func main() {
	if err := run(); err != nil {
		os.Exit(errors.ExitCode(err))
	}
}

func run() error {
	var verbose bool

	rootCmd := &cobra.Command{
		Use:   "tegata",
		Short: "Portable authenticator with tamper-evident audit logging",
		Long: `Tegata is a portable authenticator that stores encrypted credentials
on USB drives or microSD cards with optional tamper-evident audit logging
via ScalarDL Ledger.`,
		Example: `  tegata version          Show version information
  tegata code GitHub     Generate TOTP code for GitHub
  tegata --verbose code  Generate code with debug logging`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			level := slog.LevelInfo
			if verbose {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			})))
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")

	versionCmd := &cobra.Command{
		Use:     "version",
		Short:   "Print version information",
		Example: "  tegata version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("tegata %s\n", version)
		},
	}

	rootCmd.AddCommand(versionCmd)

	return rootCmd.Execute()
}
