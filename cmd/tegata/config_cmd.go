package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: i18n.T("cmd.config.short"),
	}

	showCmd := &cobra.Command{
		Use:     "show",
		Short:   i18n.T("cmd.config.show.short"),
		Args:    cobra.NoArgs,
		Example: i18n.T("cmd.config.show.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			vaultPath, err := resolveVaultPath(cmd)
			if err == nil {
				dir = vaultDir(vaultPath)
			} else {
				if cwd, cwdErr := os.Getwd(); cwdErr == nil {
					dir = cwd
				}
			}

			cfg, loadErr := config.Load(dir)
			hasFile := loadErr == nil

			fmt.Print(config.FormatEffective(cfg, hasFile, i18n.T("cmd.config.show.default")))
			return nil
		},
	}

	setCmd := &cobra.Command{
		Use:     "set <key> <value>",
		Short:   i18n.T("cmd.config.set.short"),
		Args:    cobra.ExactArgs(2),
		Example: i18n.T("cmd.config.set.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			dir := "."
			vaultPath, err := resolveVaultPath(cmd)
			if err == nil {
				dir = vaultDir(vaultPath)
			} else {
				if cwd, cwdErr := os.Getwd(); cwdErr == nil {
					dir = cwd
				}
			}

			switch key {
			case "audit.auto_start":
				var autoStart bool
				switch strings.ToLower(value) {
				case "true":
					autoStart = true
				case "false":
					autoStart = false
				default:
					return fmt.Errorf("%s", i18n.Tf("cmd.config.set.error.invalidBool", map[string]any{"Value": value}))
				}

				cfg, err := config.Load(dir)
				if err != nil {
					return fmt.Errorf("%s: %w", i18n.T("cmd.config.set.error.loadConfig"), err)
				}
				cfg.Audit.AutoStart = autoStart
				if err := config.WriteAuditSection(dir, cfg.Audit); err != nil {
					return fmt.Errorf("%s: %w", i18n.T("cmd.config.set.error.writeConfig"), err)
				}
				fmt.Print(i18n.Tf("cmd.config.set.success.autoStart", map[string]any{"Value": value}))
				return nil

			case "language":
				// Accept both short codes (en, ja) and full BCP 47 tags (en-US, ja-JP).
				lang := normalizeLangFlag(value)
				valid := false
				for _, supported := range i18n.SupportedLanguages {
					if strings.EqualFold(lang, supported) {
						lang = supported // use canonical casing
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("%s", i18n.Tf("cmd.config.set.error.invalidLanguage", map[string]any{"Value": value}))
				}
				if err := config.WriteLanguage(dir, lang); err != nil {
					return fmt.Errorf("%s: %w", i18n.T("cmd.config.set.error.writeConfig"), err)
				}
				// Switch to the new language before printing the confirmation so
				// the success message is shown in the language that was just set.
				i18n.Init(lang)
				fmt.Print(i18n.Tf("cmd.config.set.success.language", map[string]any{"Value": lang}))
				return nil

			default:
				return fmt.Errorf("%s", i18n.Tf("cmd.config.set.error.unknownKey", map[string]any{"Key": key}))
			}
		},
	}

	configCmd.AddCommand(showCmd)
	configCmd.AddCommand(setCmd)
	return configCmd
}
