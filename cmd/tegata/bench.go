package main

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
)

func newBenchCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "bench",
		Short:   i18n.T("cmd.bench.short"),
		Args:    cobra.NoArgs,
		Example: i18n.T("cmd.bench.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := crypto.DefaultParams

			salt := make([]byte, 32)
			if _, err := rand.Read(salt); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.bench.error.salt"), err)
			}
			passphrase := make([]byte, 16)
			if _, err := rand.Read(passphrase); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.bench.error.passphrase"), err)
			}

			fmt.Print(i18n.Tf("cmd.bench.header", map[string]any{
				"T": params.Time,
				"M": params.Memory / 1024,
				"P": params.Threads,
			}))

			var total time.Duration
			const runs = 3

			for i := 1; i <= runs; i++ {
				start := time.Now()
				key := crypto.DeriveKey(passphrase, salt, params)
				elapsed := time.Since(start)
				key.Destroy()
				total += elapsed
				fmt.Print(i18n.Tf("cmd.bench.run", map[string]any{"N": i, "Ms": elapsed.Milliseconds()}))
			}

			avg := total / runs
			fmt.Print(i18n.Tf("cmd.bench.average", map[string]any{"Ms": avg.Milliseconds()}))
			fmt.Print(i18n.Tf("cmd.bench.result", map[string]any{"Ms": avg.Milliseconds()}))
			fmt.Println(i18n.T("cmd.bench.target"))

			if avg > 3*time.Second {
				fmt.Println()
				fmt.Println(i18n.T("cmd.bench.warn.slow"))
			}

			return nil
		},
	}
}
