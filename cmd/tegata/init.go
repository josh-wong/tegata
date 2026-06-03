package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/josh-wong/tegata/internal/vault"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "init [path]",
		Short:   i18n.T("cmd.init.short"),
		Long:    i18n.T("cmd.init.long"),
		Args:    cobra.MaximumNArgs(1),
		Example: i18n.T("cmd.init.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("%s", i18n.Tf("cmd.init.error.getWD", map[string]any{"Err": err}))
			}
			if len(args) > 0 {
				dir = args[0]
			}

			vaultPath := filepath.Join(dir, vaultFilename)

			if !isRemovablePath(dir) {
				fmt.Fprintln(os.Stderr, i18n.T("cmd.init.warn.notRemovable"))
				fmt.Fprintln(os.Stderr, i18n.T("cmd.init.info.removableAdvice"))
				fmt.Fprintln(os.Stderr, i18n.T("cmd.init.info.physicalSep"))
				fmt.Fprintln(os.Stderr)
			}

			if _, err := os.Stat(vaultPath); err == nil {
				return fmt.Errorf("%s", i18n.Tf("cmd.init.error.alreadyExists", map[string]any{"Path": vaultPath}))
			}

			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("%s", i18n.Tf("cmd.init.error.createDir", map[string]any{"Err": err}))
			}

			passphrase, err := promptNewPassphrase()
			if err != nil {
				return err
			}
			defer zeroBytes(passphrase)

			recoveryKey, err := vault.Create(vaultPath, passphrase, crypto.DefaultParams)
			if err != nil {
				return fmt.Errorf("%s", i18n.Tf("cmd.init.error.createVault", map[string]any{"Err": err}))
			}

			if err := config.WriteDefaults(dir); err != nil {
				fmt.Fprintf(os.Stderr, "%s",
					i18n.Tf("cmd.init.warn.defaultConfig", map[string]any{"Err": err}))
			}

			fmt.Print(i18n.Tf("cmd.init.success", map[string]any{"Path": vaultPath}))
			fmt.Println(i18n.T("cmd.init.recoveryKeyHeader"))
			fmt.Print(i18n.Tf("cmd.init.recoveryKeyDisplay", map[string]any{"Key": recoveryKey}))
			fmt.Println(i18n.T("cmd.init.recoveryKeyAdvice"))

			fmt.Fprintf(os.Stderr, "%s", i18n.T("cmd.init.prompt.enableAudit"))
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				answer := strings.TrimSpace(scanner.Text())
				if strings.EqualFold(answer, "y") {
					runInitAudit(vaultPath, dir, passphrase)
				}
			}

			return nil
		},
	}
}

// runInitAudit runs the full Docker audit setup immediately after vault
// creation. It opens and unlocks the vault to derive the stable entity ID,
// then calls audit.SetupStack. The vault manager is kept open so that the
// HMAC secret key and ledger volume key can be stored via the onRegistered
// callback before SetupStack returns. Errors are printed to stderr and the
// user is directed to run 'tegata ledger start' to retry.
func runInitAudit(vaultPath, dir string, passphrase []byte) {
	fmt.Fprintln(os.Stderr, i18n.T("cmd.init.info.settingUpAudit"))

	mgr, err := vault.Open(vaultPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s",
			i18n.Tf("cmd.init.error.auditSetup", map[string]any{"Err": err}))
		return
	}
	if err := mgr.Unlock(passphrase); err != nil {
		mgr.Close()
		fmt.Fprintf(os.Stderr, "%s",
			i18n.Tf("cmd.init.error.auditSetup", map[string]any{"Err": err}))
		return
	}
	defer mgr.Close()
	vaultID := mgr.VaultID()

	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s",
			i18n.Tf("cmd.init.error.auditSetup", map[string]any{"Err": err}))
		return
	}
	composeDir := audit.ComposeDirForVault(u.HomeDir, vaultID)

	bundleFS, err := fs.Sub(dockerBundle, "docker-bundle")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s",
			i18n.Tf("cmd.init.error.auditSetup", map[string]any{"Err": err}))
		return
	}

	progressFn := func(msg string) { fmt.Fprintln(os.Stderr, msg) }

	onRegistered := func(auditCfg config.AuditConfig) error {
		if auditCfg.SecretKey != "" {
			if vaultErr := mgr.SetSecret("audit.secret_key", auditCfg.SecretKey); vaultErr != nil {
				return fmt.Errorf("%s",
					i18n.Tf("cmd.init.error.storeHMAC", map[string]any{"Err": vaultErr}))
			}
		}
		if len(auditCfg.LedgerVolumeKey) > 0 {
			hexKey := hex.EncodeToString(auditCfg.LedgerVolumeKey)
			if vaultErr := mgr.SetSecret("audit.ledger_volume_key", hexKey); vaultErr != nil {
				return fmt.Errorf("%s",
					i18n.Tf("cmd.init.error.storeLedgerKey", map[string]any{"Err": vaultErr}))
			}
			zeroBytes(auditCfg.LedgerVolumeKey)
		}
		auditCfg.AutoStart = true
		if writeErr := config.WriteAuditSection(dir, auditCfg); writeErr != nil {
			return fmt.Errorf("%s",
				i18n.Tf("cmd.init.error.writeAuditConfig", map[string]any{"Err": writeErr}))
		}
		return nil
	}

	if _, err := audit.SetupStack(bundleFS, composeDir, vaultID, progressFn, onRegistered); err != nil {
		fmt.Fprintf(os.Stderr, "%s",
			i18n.Tf("cmd.init.error.auditSetup", map[string]any{"Err": err}))
		return
	}

	fmt.Fprintln(os.Stderr, i18n.T("cmd.init.success.auditEnabled"))
}
