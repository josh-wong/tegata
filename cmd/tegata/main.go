// Package main provides the CLI entrypoint for Tegata.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
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
			fmt.Fprint(os.Stderr, i18n.Tf("main.error", map[string]any{"Err": err}))
		}
		os.Exit(errors.ExitCode(err))
	}
}

func run() error {
	// Initialise i18n before building the cobra tree so that Short/Long/Example
	// strings on every command are already localised.
	initI18n()

	var verbose bool
	var langFlag string

	rootCmd := &cobra.Command{
		Use:          "tegata",
		Short:        i18n.T("cmd.root.short"),
		Long:         i18n.T("cmd.root.long"),
		Example:      i18n.T("cmd.root.example"),
		SilenceUsage: true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			level := slog.LevelInfo
			if verbose {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			})))
			// Phase-2 language init: cobra has now fully parsed all flags,
			// so we can resolve the vault path properly and reload the language
			// from its tegata.toml. This corrects any mis-detection from the
			// early pre-parse in initI18n and ensures runtime output (error
			// messages, status lines, prompts) uses the vault's stored language.
			// --lang always takes priority over the stored setting.
			//
			// Limitation: cobra Short/Long/Example strings are baked in at
			// phase-1 (initI18n). If phase-2 changes the language they will
			// stay in the phase-1 language for this invocation's --help output.
			if langFlag != "" {
				i18n.Init(normalizeLangFlag(langFlag))
			} else if vaultPath, err := resolveVaultPath(cmd); err == nil {
				if cfg, loadErr := config.Load(vaultDir(vaultPath)); loadErr == nil {
					i18n.Init(cfg.Language)
				}
			}
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, i18n.T("cmd.root.flag.verbose"))
	rootCmd.PersistentFlags().String("vault", "", i18n.T("cmd.root.flag.vault"))
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "", i18n.T("cmd.root.flag.lang"))
	// Register -h/--help as a persistent flag before cobra's InitDefaultHelpFlag
	// runs. Cobra checks if the flag already exists and skips its own version,
	// so this ensures the translated description is used on every subcommand.
	rootCmd.PersistentFlags().BoolP("help", "h", false, i18n.T("flag.help"))

	versionCmd := &cobra.Command{
		Use:     "version",
		Short:   i18n.T("cmd.version.short"),
		Example: i18n.T("cmd.version.example"),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(i18n.Tf("cmd.version.output", map[string]any{"Version": version}))
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

	// cobra adds "completion" during Execute() and "help" during AddCommand, so
	// we patch them inside the help function — the only hook that fires before
	// the help template renders, regardless of how help was triggered.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		for _, c := range cmd.Root().Commands() {
			switch c.Name() {
			case "completion":
				c.Short = i18n.T("cmd.completion.short")
			case "help":
				c.Short = i18n.T("cmd.help.short")
			}
		}
		defaultHelp(cmd, args)
	})

	return rootCmd.Execute()
}

// initI18n resolves the active language and initialises the global localizer.
// Priority: --lang flag in os.Args > vault config > system locale > "en-US".
func initI18n() {
	lang := normalizeLangFlag(preParseLangFlag())
	if lang == "" {
		lang = loadConfigLang()
	}
	if lang == "" {
		lang = i18n.DetectFromEnv()
	}
	i18n.Init(lang)
}

// normalizeLangFlag converts any accepted input form to the canonical lowercase
// code the app uses. Accepts short forms (en, ja), uppercase BCP 47 (en-US,
// ja-JP), and the canonical lowercase form (en-us, ja-jp). Unknown values are
// passed through unchanged so Init can fall back to American English.
func normalizeLangFlag(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "en", i18n.LangEnUS:
		return i18n.LangEnUS
	case "ja", i18n.LangJaJP:
		return i18n.LangJaJP
	default:
		return code
	}
}

// preParseLangFlag scans os.Args for --lang or --lang=<value> before cobra
// has parsed flags, so the language is available when constructing commands.
// Values are unquoted so that --lang="en-us" and --lang=en-us are equivalent.
func preParseLangFlag() string {
	args := os.Args[1:]
	for i, arg := range args {
		if strings.HasPrefix(arg, "--lang=") {
			val := strings.TrimPrefix(arg, "--lang=")
			return strings.Trim(val, `"'`)
		}
		if arg == "--lang" && i+1 < len(args) {
			return strings.Trim(args[i+1], `"'`)
		}
	}
	return ""
}

// loadConfigLang attempts to find a vault directory from --vault in os.Args
// (or the current directory) and loads the language from tegata.toml.
func loadConfigLang() string {
	dir := preParseVaultDir()
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return ""
		}
		dir = cwd
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return ""
	}
	return normalizeLangFlag(cfg.Language)
}

// preParseVaultDir scans os.Args for --vault or --vault=<value>.
// vaultDir is defined in helpers.go.
func preParseVaultDir() string {
	args := os.Args[1:]
	for i, arg := range args {
		if strings.HasPrefix(arg, "--vault=") {
			p := strings.TrimPrefix(arg, "--vault=")
			return vaultDir(p)
		}
		if arg == "--vault" && i+1 < len(args) {
			return vaultDir(args[i+1])
		}
	}
	return ""
}
