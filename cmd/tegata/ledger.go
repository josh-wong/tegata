package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"golang.org/x/term"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	tegerrors "github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/josh-wong/tegata/internal/vault"
	"github.com/spf13/cobra"
)

// auditTOMLExample is the canonical inline TOML block shown in help text and
// error output. Defined once so both surfaces stay in sync. secret_key is
// intentionally omitted — it is stored in the encrypted vault, not in tegata.toml.
const auditTOMLExample = `  [audit]
  enabled           = true
  server            = "127.0.0.1:50051"
  privileged_server = "127.0.0.1:50052"
  entity_id         = "tegata-client"
  key_version       = 1
  insecure          = true  # set to false for remote or production servers`

// newLedgerCmd returns the 'tegata ledger' command with its subcommands.
func newLedgerCmd() *cobra.Command {
	ledgerCmd := &cobra.Command{
		Use:   "ledger",
		Short: i18n.T("cmd.ledger.short"),
		Long:  i18n.T("cmd.ledger.long"),
	}

	ledgerCmd.AddCommand(newLedgerSetupCmd())
	ledgerCmd.AddCommand(newLedgerStartCmd())
	ledgerCmd.AddCommand(newLedgerStopCmd())
	return ledgerCmd
}

// newLedgerSetupCmd returns the 'tegata ledger setup' command.
func newLedgerSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "setup",
		Short:   i18n.T("cmd.ledger.setup.short"),
		Long:    i18n.T("cmd.ledger.setup.long"),
		Example: i18n.T("cmd.ledger.setup.example"),
		Args:    cobra.NoArgs,
		RunE:    runLedgerSetup,
	}
}

func runLedgerSetup(cmd *cobra.Command, _ []string) error {
	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}
	dir := vaultDir(vaultPath)

	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.loadConfig", map[string]any{"Err": err}))
	}

	if !cfg.Audit.Enabled {
		fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.error.auditNotEnabled"))
		fmt.Fprintln(os.Stderr, auditTOMLExample)
		fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.info.dockerAlternative"))
		return nil
	}

	if cfg.Audit.SecretKey == "" {
		fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.info.loadingSecret"))
		secret, err := unlockVaultForSecret(cmd)
		if err != nil {
			return err
		}
		cfg.Audit.SecretKey = secret
	}
	defer func() { cfg.Audit.SecretKey = "" }()

	fmt.Fprintf(os.Stderr, "%s",
		i18n.Tf("cmd.ledger.info.connecting",
			map[string]any{"Server": cfg.Audit.Server, "Privileged": cfg.Audit.PrivilegedServer}))
	if cfg.Audit.Insecure {
		fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.warn.insecure"))
	}
	client, err := audit.NewClientFromConfig(cfg.Audit)
	if err != nil {
		return fmt.Errorf("%w: connecting to ledger: %s", tegerrors.ErrNetworkFailed, err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Fprintf(os.Stderr, "%s",
		i18n.Tf("cmd.ledger.info.registering",
			map[string]any{"EntityID": cfg.Audit.EntityID, "KeyVersion": cfg.Audit.KeyVersion}))
	if err := client.RegisterSecret(ctx, cfg.Audit.EntityID, cfg.Audit.KeyVersion, cfg.Audit.SecretKey); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.success.registered"))

	fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.info.verifyingConnectivity"))
	if err := client.Ping(ctx); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.info.verifyingContracts"))
	if err := verifyContracts(ctx, client); err != nil {
		composeDir := "~/.tegata/docker"
		if u, err := user.Current(); err == nil {
			composeDir = filepath.Join(u.HomeDir, ".tegata", "docker")
		}
		fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.error.contractsNotRegistered"))
		fmt.Fprintf(os.Stderr, "%s",
			i18n.Tf("cmd.ledger.info.registerContracts", map[string]any{"Path": composeDir}))
		fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.info.signatureNote1"))
		fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.info.signatureNote2"))
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.contractVerification", map[string]any{"Err": err}))
	}

	fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.info.storingSecret"))
	mgr, err := vault.Open(vaultPath)
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.openVaultForSecret", map[string]any{"Err": err}))
	}
	defer mgr.Close()

	if err := mgr.Unlock([]byte(os.Getenv("TEGATA_VAULT_PASSPHRASE"))); err != nil {
		fmt.Fprint(os.Stderr, i18n.T("cmd.ledger.prompt.passphrase"))
		passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.readPassphrase", map[string]any{"Err": err}))
		}
		fmt.Fprintln(os.Stderr)
		defer func() {
			for i := range passBytes {
				passBytes[i] = 0
			}
		}()
		if err := mgr.Unlock(passBytes); err != nil {
			return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.unlockVault", map[string]any{"Err": err}))
		}
	}

	if err := mgr.SetSecret("audit.secret_key", cfg.Audit.SecretKey); err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.storeSecret", map[string]any{"Err": err}))
	}

	fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.success.setupComplete"))
	return nil
}

// verifyContracts attempts a test Put to confirm that the predefined HashStore
// contracts are registered on the ScalarDL instance.
func verifyContracts(ctx context.Context, client audit.Client) error {
	return client.Put(ctx, audit.SetupTestObjectID, "0000000000000000000000000000000000000000000000000000000000000000")
}

// newLedgerStartCmd returns the 'tegata ledger start' command.
func newLedgerStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "start",
		Short:   i18n.T("cmd.ledger.start.short"),
		Long:    i18n.T("cmd.ledger.start.long"),
		Example: i18n.T("cmd.ledger.start.example"),
		Args:    cobra.NoArgs,
		RunE:    runLedgerStart,
	}
}

func runLedgerStart(cmd *cobra.Command, _ []string) error {
	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}
	dir := filepath.Dir(vaultPath)

	passphraseBytes, err := promptPassphrase(i18n.T("cmd.ledger.prompt.vaultPass"))
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.readPassphrase", map[string]any{"Err": err}))
	}
	mgr, err := vault.Open(vaultPath)
	if err != nil {
		zeroBytes(passphraseBytes)
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.openVault", map[string]any{"Err": err}))
	}
	if err := mgr.Unlock(passphraseBytes); err != nil {
		zeroBytes(passphraseBytes)
		mgr.Close()
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.unlockVault", map[string]any{"Err": err}))
	}
	zeroBytes(passphraseBytes)
	defer mgr.Close()

	// Strip the docker-bundle/ prefix from the embed.FS so SetupStack and
	// EnsureStack receive an FS rooted at the docker-compose.yml level.
	bundleFS, err := fs.Sub(dockerBundle, "docker-bundle")
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.accessBundle", map[string]any{"Err": err}))
	}

	progressFn := func(msg string) {
		fmt.Fprintln(os.Stderr, msg)
	}

	// If the audit stack is already configured, load the existing keys and
	// start/ensure the stack. Re-running SetupStack would generate a new key
	// and fail to decrypt any existing ledger-data.enc archive.
	if cfg, loadErr := config.Load(dir); loadErr == nil && cfg.Audit.Enabled && cfg.Audit.DockerComposePath != "" {
		if volumeKeyHex := mgr.GetSecret("audit.ledger_volume_key"); volumeKeyHex != "" {
			if key, decErr := hex.DecodeString(volumeKeyHex); decErr == nil {
				cfg.Audit.LedgerVolumeKey = key
				defer zeroBytes(cfg.Audit.LedgerVolumeKey)
			}
		}
		if secretKey := mgr.GetSecret("audit.secret_key"); secretKey != "" {
			cfg.Audit.SecretKey = secretKey
			defer func() { cfg.Audit.SecretKey = "" }()
		}
		return audit.EnsureStack(cfg.Audit, bundleFS, progressFn)
	}

	vaultID := mgr.VaultID()

	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.resolveHome", map[string]any{"Err": err}))
	}
	composeDir := audit.ComposeDirForVault(u.HomeDir, vaultID)

	// onRegistered stores the generated HMAC secret and ledger volume key in the
	// encrypted vault and writes the [audit] section to tegata.toml. The secrets
	// are stored FIRST to ensure transactional consistency: if vault storage fails,
	// the config file is not modified.
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
		return err
	}

	fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.success.started"))
	return nil
}

// newLedgerStopCmd returns the 'tegata ledger stop' command.
func newLedgerStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "stop",
		Short:   i18n.T("cmd.ledger.stop.short"),
		Long:    i18n.T("cmd.ledger.stop.long"),
		Example: i18n.T("cmd.ledger.stop.example"),
		Args:    cobra.NoArgs,
		RunE:    runLedgerStop,
	}
}

func runLedgerStop(cmd *cobra.Command, _ []string) error {
	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}
	dir := filepath.Dir(vaultPath)

	passphraseBytes, err := promptPassphrase(i18n.T("cmd.ledger.prompt.vaultPass"))
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.readPassphrase", map[string]any{"Err": err}))
	}
	mgr, err := vault.Open(vaultPath)
	if err != nil {
		zeroBytes(passphraseBytes)
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.openVault", map[string]any{"Err": err}))
	}
	if err := mgr.Unlock(passphraseBytes); err != nil {
		zeroBytes(passphraseBytes)
		mgr.Close()
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.unlockVault", map[string]any{"Err": err}))
	}
	zeroBytes(passphraseBytes)

	cfg, err := config.Load(dir)
	if err != nil {
		mgr.Close()
		return fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.loadConfig", map[string]any{"Err": err}))
	}

	if cfg.Audit.DockerComposePath == "" {
		mgr.Close()
		return fmt.Errorf("%s", i18n.T("cmd.ledger.error.dockerNotFound"))
	}

	if err := audit.StopStack(cfg.Audit.DockerComposePath, cfg.Audit.DockerProjectName); err != nil {
		mgr.Close()
		return err
	}

	if volumeKeyHex := mgr.GetSecret("audit.ledger_volume_key"); volumeKeyHex != "" {
		if volumeKey, hexErr := hex.DecodeString(volumeKeyHex); hexErr == nil {
			defer zeroBytes(volumeKey)
			cfg.Audit.LedgerVolumeKey = volumeKey
			if lockErr := audit.LockLedgerVolume(cfg.Audit); lockErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%s",
					i18n.Tf("cmd.ledger.warn.encryptVolume", map[string]any{"Err": lockErr}))
			}
		}
	}
	mgr.Close()
	composeDir := filepath.Dir(cfg.Audit.DockerComposePath)
	clientPropsPath := filepath.Join(composeDir, "certs", "client.properties")
	if err := os.Remove(clientPropsPath); err != nil && !os.IsNotExist(err) {
		_, _ = fmt.Fprintf(os.Stderr, "%s",
			i18n.Tf("cmd.ledger.warn.deleteClientProps", map[string]any{"Err": err}))
	}

	fmt.Fprintln(os.Stderr, i18n.T("cmd.ledger.success.stopped"))
	return nil
}
