// Package main provides the CLI entrypoint for Tegata.
package main

import (
	"fmt"
	"log/slog"
	"os"

	runewidth "github.com/mattn/go-runewidth"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/spf13/cobra"
)

func init() {
	// Force narrow (1-column) East Asian Width for Unicode ambiguous-width
	// characters such as ─ (U+2500) and ▸ (U+25B8). On Windows systems with
	// a CJK locale (e.g., Japanese Shift-JIS code page 932), go-runewidth's
	// platform init sets EastAsianWidth=true, which causes these characters to
	// measure as 2 display columns. Lipgloss uses go-runewidth for all string
	// width calculations, so the TUI table separator and cursor indicator render
	// at double their intended width, misaligning every column in the audit
	// history view. Setting EastAsianWidth=false here runs after go-runewidth's
	// own init and forces consistent 1-column measurement on all platforms.
	runewidth.DefaultCondition.EastAsianWidth = false
}

// version is set via -ldflags "-X main.version=..." at build time.
var version = "dev"

// reportedError wraps an error that has already been printed to the user.
// main() skips printing for these so commands can control their own output order.
type reportedError struct{ err error }

func (e reportedError) Error() string { return e.err.Error() }
func (e reportedError) Unwrap() error { return e.err }

func main() {
	if err := run(); err != nil {
		var re reportedError
		if !errors.As(err, &re) {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
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
	rootCmd.PersistentFlags().String("vault", "", "path to vault file or directory")

	versionCmd := &cobra.Command{
		Use:     "version",
		Short:   "Print version information",
		Example: "  tegata version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("tegata %s\n", version)
		},
	}

	rootCmd.AddCommand(
		versionCmd,
		newUICmd(),
		newInitCmd(),
		newAddCmd(),
		newListCmd(),
		newRemoveCmd(),
		newCodeCmd(),
		newGetCmd(),
		newSignCmd(),
		newResyncCmd(),
		newBenchCmd(),
		newConfigCmd(),
		newExportCmd(),
		newImportCmd(),
		newEditCmd(),
		newTagCmd(),
		newChangePassphraseCmd(),
		newVerifyRecoveryCmd(),
		newLedgerCmd(),
		newHistoryCmd(),
		newVerifyCmd(),
	)

	return rootCmd.Execute()
}
